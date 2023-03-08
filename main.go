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
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	crtlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/fluxcd/pkg/runtime/acl"
	"github.com/fluxcd/pkg/runtime/client"
	helper "github.com/fluxcd/pkg/runtime/controller"
	feathelper "github.com/fluxcd/pkg/runtime/features"
	"github.com/fluxcd/pkg/runtime/leaderelection"
	"github.com/fluxcd/pkg/runtime/logger"
	"github.com/fluxcd/pkg/runtime/pprof"
	"github.com/fluxcd/pkg/runtime/probes"

	apiv1 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/notification-controller/controllers"
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
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		eventsAddr            string
		receiverAddr          string
		healthAddr            string
		metricsAddr           string
		concurrent            int
		watchAllNamespaces    bool
		rateLimitInterval     time.Duration
		clientOptions         client.Options
		logOptions            logger.Options
		leaderElectionOptions leaderelection.Options
		aclOptions            acl.Options
		rateLimiterOptions    helper.RateLimiterOptions
		featureGates          feathelper.FeatureGates
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&eventsAddr, "events-addr", ":9090", "The address the event endpoint binds to.")
	flag.StringVar(&healthAddr, "health-addr", ":9440", "The address the health endpoint binds to.")
	flag.StringVar(&receiverAddr, "receiverAddr", ":9292", "The address the webhook receiver endpoint binds to.")
	flag.IntVar(&concurrent, "concurrent", 4, "The number of concurrent notification reconciles.")
	flag.BoolVar(&watchAllNamespaces, "watch-all-namespaces", true,
		"Watch for custom resources in all namespaces, if set to false it will only watch the runtime namespace.")
	flag.DurationVar(&rateLimitInterval, "rate-limit-interval", 5*time.Minute, "Interval in which rate limit has effect.")

	clientOptions.BindFlags(flag.CommandLine)
	logOptions.BindFlags(flag.CommandLine)
	leaderElectionOptions.BindFlags(flag.CommandLine)
	aclOptions.BindFlags(flag.CommandLine)
	rateLimiterOptions.BindFlags(flag.CommandLine)
	featureGates.BindFlags(flag.CommandLine)

	flag.Parse()

	logger.SetLogger(logger.NewLogger(logOptions))

	if err := featureGates.WithLogger(setupLog).SupportedFeatures(features.FeatureGates()); err != nil {
		setupLog.Error(err, "unable to load feature gates")
		os.Exit(1)
	}

	watchNamespace := ""
	if !watchAllNamespaces {
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
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                        scheme,
		MetricsBindAddress:            metricsAddr,
		HealthProbeBindAddress:        healthAddr,
		Port:                          9443,
		LeaderElection:                leaderElectionOptions.Enable,
		LeaderElectionReleaseOnCancel: leaderElectionOptions.ReleaseOnCancel,
		LeaseDuration:                 &leaderElectionOptions.LeaseDuration,
		RenewDeadline:                 &leaderElectionOptions.RenewDeadline,
		RetryPeriod:                   &leaderElectionOptions.RetryPeriod,
		LeaderElectionID:              fmt.Sprintf("%s-leader-election", controllerName),
		Namespace:                     watchNamespace,
		Logger:                        ctrl.Log,
		ClientDisableCacheFor:         disableCacheFor,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	probes.SetupChecks(mgr, setupLog)
	pprof.SetupHandlers(mgr, setupLog)

	metricsH := helper.MustMakeMetrics(mgr)

	if err = (&controllers.ProviderReconciler{
		Client:         mgr.GetClient(),
		ControllerName: controllerName,
		Metrics:        metricsH,
		EventRecorder:  mgr.GetEventRecorderFor(controllerName),
	}).SetupWithManagerAndOptions(mgr, controllers.ProviderReconcilerOptions{
		MaxConcurrentReconciles: concurrent,
		RateLimiter:             helper.GetRateLimiter(rateLimiterOptions),
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Provider")
		os.Exit(1)
	}
	if err = (&controllers.AlertReconciler{
		Client:         mgr.GetClient(),
		ControllerName: controllerName,
		Metrics:        metricsH,
		EventRecorder:  mgr.GetEventRecorderFor(controllerName),
	}).SetupWithManagerAndOptions(mgr, controllers.AlertReconcilerOptions{
		MaxConcurrentReconciles: concurrent,
		RateLimiter:             helper.GetRateLimiter(rateLimiterOptions),
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Alert")
		os.Exit(1)
	}
	if err = (&controllers.ReceiverReconciler{
		Client:         mgr.GetClient(),
		ControllerName: controllerName,
		Metrics:        metricsH,
		EventRecorder:  mgr.GetEventRecorderFor(controllerName),
	}).SetupWithManagerAndOptions(mgr, controllers.ReceiverReconcilerOptions{
		MaxConcurrentReconciles: concurrent,
		RateLimiter:             helper.GetRateLimiter(rateLimiterOptions),
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Receiver")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	ctx := ctrl.SetupSignalHandler()
	store, err := memorystore.New(&memorystore.Config{
		Interval: rateLimitInterval,
	})

	setupLog.Info("starting event server", "addr", eventsAddr)
	eventMdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix:   "gotk_event",
			Registry: crtlmetrics.Registry,
		}),
	})
	eventServer := server.NewEventServer(eventsAddr, ctrl.Log, mgr.GetClient(), aclOptions.NoCrossNamespaceRefs)
	go eventServer.ListenAndServe(ctx.Done(), eventMdlw, store)

	setupLog.Info("starting webhook receiver server", "addr", receiverAddr)
	receiverServer := server.NewReceiverServer(receiverAddr, ctrl.Log, mgr.GetClient())
	receiverMdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix:   "gotk_receiver",
			Registry: crtlmetrics.Registry,
		}),
	})
	go receiverServer.ListenAndServe(ctx.Done(), receiverMdlw)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
