/*
Copyright 2022 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/sethvargo/go-limiter/memorystore"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	flag "github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
	pkgcache "github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/runtime/acl"
	"github.com/fluxcd/pkg/runtime/client"
	runtimeCtrl "github.com/fluxcd/pkg/runtime/controller"
	feathelper "github.com/fluxcd/pkg/runtime/features"
	"github.com/fluxcd/pkg/runtime/leaderelection"
	"github.com/fluxcd/pkg/runtime/logger"
	"github.com/fluxcd/pkg/runtime/metrics"
	"github.com/fluxcd/pkg/runtime/pprof"
	"github.com/fluxcd/pkg/runtime/probes"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1b2 "github.com/fluxcd/notification-controller/api/v1beta2"
	apiv1b3 "github.com/fluxcd/notification-controller/api/v1beta3"
	"github.com/fluxcd/notification-controller/internal/controller"
	"github.com/fluxcd/notification-controller/internal/features"
	"github.com/fluxcd/notification-controller/internal/server"
	// +kubebuilder:scaffold:imports
)

const controllerName = "notification-controller"

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = apiv1.AddToScheme(scheme)
	_ = apiv1b2.AddToScheme(scheme)
	_ = apiv1b3.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	const (
		tokenCacheDefaultMaxSize = 100
	)

	var (
		eventsAddr            string
		receiverAddr          string
		healthAddr            string
		metricsAddr           string
		concurrent            int
		rateLimitInterval     time.Duration
		clientOptions         client.Options
		logOptions            logger.Options
		leaderElectionOptions leaderelection.Options
		aclOptions            acl.Options
		rateLimiterOptions    runtimeCtrl.RateLimiterOptions
		featureGates          feathelper.FeatureGates
		exportHTTPPathMetrics bool
		tokenCacheOptions     pkgcache.TokenFlags
		watchOptions          runtimeCtrl.WatchOptions
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&eventsAddr, "events-addr", ":9090", "The address the event endpoint binds to.")
	flag.StringVar(&healthAddr, "health-addr", ":9440", "The address the health endpoint binds to.")
	flag.StringVar(&receiverAddr, "receiverAddr", ":9292", "The address the webhook receiver endpoint binds to.")
	flag.IntVar(&concurrent, "concurrent", 4, "The number of concurrent notification reconciles.")
	flag.DurationVar(&rateLimitInterval, "rate-limit-interval", 5*time.Minute, "Interval in which rate limit has effect.")
	flag.BoolVar(&exportHTTPPathMetrics, "export-http-path-metrics", false, "When enabled, the requests full path is included in the HTTP server metrics (risk as high cardinality")
	// After implementing --watch-label-selector the following two bindings can be replaced by watchOptions.BindFlags().
	flag.BoolVar(&watchOptions.AllNamespaces, "watch-all-namespaces", true,
		"Watch for custom resources in all namespaces, if set to false it will only watch the runtime namespace.")
	flag.CommandLine.StringVar(&watchOptions.ConfigsLabelSelector, "watch-configs-label-selector", meta.LabelKeyWatch+"="+meta.LabelValueWatchEnabled,
		"Watch for ConfigMaps and Secrets with matching labels.")

	clientOptions.BindFlags(flag.CommandLine)
	logOptions.BindFlags(flag.CommandLine)
	leaderElectionOptions.BindFlags(flag.CommandLine)
	aclOptions.BindFlags(flag.CommandLine)
	rateLimiterOptions.BindFlags(flag.CommandLine)
	featureGates.BindFlags(flag.CommandLine)
	tokenCacheOptions.BindFlags(flag.CommandLine, tokenCacheDefaultMaxSize)

	flag.Parse()

	logger.SetLogger(logger.NewLogger(logOptions))

	if err := featureGates.WithLogger(setupLog).SupportedFeatures(features.FeatureGates()); err != nil {
		setupLog.Error(err, "unable to load feature gates")
		os.Exit(1)
	}

	switch enabled, err := features.Enabled(auth.FeatureGateObjectLevelWorkloadIdentity); {
	case err != nil:
		setupLog.Error(err, "unable to check feature gate "+auth.FeatureGateObjectLevelWorkloadIdentity)
		os.Exit(1)
	case enabled:
		auth.EnableObjectLevelWorkloadIdentity()
	}

	watchNamespace := ""
	if !watchOptions.AllNamespaces {
		watchNamespace = os.Getenv("RUNTIME_NAMESPACE")
	}

	var disableCacheFor []ctrlclient.Object
	shouldCache, err := features.Enabled(features.CacheSecretsAndConfigMaps)
	if err != nil {
		setupLog.Error(err, "unable to check feature gate "+features.CacheSecretsAndConfigMaps)
		os.Exit(1)
	}
	if !shouldCache {
		disableCacheFor = append(disableCacheFor, &corev1.Secret{}, &corev1.ConfigMap{})
	}

	restConfig := client.GetConfigOrDie(clientOptions)
	mgrConfig := ctrl.Options{
		Scheme:                        scheme,
		HealthProbeBindAddress:        healthAddr,
		LeaderElection:                leaderElectionOptions.Enable,
		LeaderElectionReleaseOnCancel: leaderElectionOptions.ReleaseOnCancel,
		LeaseDuration:                 &leaderElectionOptions.LeaseDuration,
		RenewDeadline:                 &leaderElectionOptions.RenewDeadline,
		RetryPeriod:                   &leaderElectionOptions.RetryPeriod,
		LeaderElectionID:              fmt.Sprintf("%s-leader-election", controllerName),
		Logger:                        ctrl.Log,
		Controller: config.Controller{
			RecoverPanic:            ptr.To(true),
			MaxConcurrentReconciles: concurrent,
		},
		Client: ctrlclient.Options{
			Cache: &ctrlclient.CacheOptions{
				DisableFor: disableCacheFor,
			},
		},
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			ExtraHandlers: pprof.GetHandlers(),
		},
	}

	if watchNamespace != "" {
		mgrConfig.Cache = ctrlcache.Options{
			DefaultNamespaces: map[string]ctrlcache.Config{
				watchNamespace: {},
			},
		}
	}

	mgr, err := ctrl.NewManager(restConfig, mgrConfig)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	probes.SetupChecks(mgr, setupLog)

	metricsH := runtimeCtrl.NewMetrics(mgr, metrics.MustMakeRecorder(), apiv1.NotificationFinalizer)

	var tokenCache *pkgcache.TokenCache
	if tokenCacheOptions.MaxSize > 0 {
		var err error
		tokenCache, err = pkgcache.NewTokenCache(tokenCacheOptions.MaxSize,
			pkgcache.WithMaxDuration(tokenCacheOptions.MaxDuration),
			pkgcache.WithMetricsRegisterer(ctrlmetrics.Registry),
			pkgcache.WithMetricsPrefix("gotk_token_"))
		if err != nil {
			setupLog.Error(err, "unable to create token cache")
			os.Exit(1)
		}
	}

	watchConfigsPredicate, err := runtimeCtrl.GetWatchConfigsPredicate(watchOptions)
	if err != nil {
		setupLog.Error(err, "unable to configure watch configs label selector for controller")
		os.Exit(1)
	}

	if err = (&controller.ProviderReconciler{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(controllerName),
		TokenCache:    tokenCache,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Provider")
		os.Exit(1)
	}

	if err = (&controller.AlertReconciler{
		Client:         mgr.GetClient(),
		ControllerName: controllerName,
		EventRecorder:  mgr.GetEventRecorderFor(controllerName),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Alert")
		os.Exit(1)
	}

	if err = (&controller.ReceiverReconciler{
		Client:         mgr.GetClient(),
		ControllerName: controllerName,
		Metrics:        metricsH,
		EventRecorder:  mgr.GetEventRecorderFor(controllerName),
	}).SetupWithManagerAndOptions(mgr, controller.ReceiverReconcilerOptions{
		RateLimiter:           runtimeCtrl.GetRateLimiter(rateLimiterOptions),
		WatchConfigsPredicate: watchConfigsPredicate,
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Receiver")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	ctx := ctrl.SetupSignalHandler()
	store, err := memorystore.New(&memorystore.Config{
		Interval: rateLimitInterval,
	})
	if err != nil {
		setupLog.Error(err, "unable to create middleware store")
		os.Exit(1)
	}

	setupLog.Info("starting event server", "addr", eventsAddr)
	eventMdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix:   "gotk_event",
			Registry: ctrlmetrics.Registry,
		}),
	})
	eventServer := server.NewEventServer(eventsAddr, ctrl.Log, mgr.GetClient(), mgr.GetEventRecorderFor(controllerName), aclOptions.NoCrossNamespaceRefs, exportHTTPPathMetrics, tokenCache)
	go eventServer.ListenAndServe(ctx.Done(), eventMdlw, store)

	setupLog.Info("starting webhook receiver server", "addr", receiverAddr)
	receiverServer := server.NewReceiverServer(receiverAddr, ctrl.Log, mgr.GetClient(), aclOptions.NoCrossNamespaceRefs, exportHTTPPathMetrics)
	receiverMdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix:   "gotk_receiver",
			Registry: ctrlmetrics.Registry,
		}),
	})
	go receiverServer.ListenAndServe(ctx.Done(), receiverMdlw)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
