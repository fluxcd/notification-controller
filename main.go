/*
Copyright 2020 The Flux authors

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
	"os"

	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	crtlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
	"github.com/fluxcd/pkg/runtime/client"
	"github.com/fluxcd/pkg/runtime/logger"
	"github.com/fluxcd/pkg/runtime/metrics"
	"github.com/fluxcd/pkg/runtime/probes"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"

	"github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/controllers"
	"github.com/fluxcd/notification-controller/internal/server"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = v1beta1.AddToScheme(scheme)
	_ = sourcev1.AddToScheme(scheme)
	_ = imagev1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		eventsAddr           string
		receiverAddr         string
		healthAddr           string
		metricsAddr          string
		enableLeaderElection bool
		concurrent           int
		watchAllNamespaces   bool
		clientOptions        client.Options
		logOptions           logger.Options
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&eventsAddr, "events-addr", ":9090", "The address the event endpoint binds to.")
	flag.StringVar(&healthAddr, "health-addr", ":9440", "The address the health endpoint binds to.")
	flag.StringVar(&receiverAddr, "receiverAddr", ":9292", "The address the webhook receiver endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&concurrent, "concurrent", 4, "The number of concurrent notification reconciles.")
	flag.BoolVar(&watchAllNamespaces, "watch-all-namespaces", true,
		"Watch for custom resources in all namespaces, if set to false it will only watch the runtime namespace.")
	flag.Bool("log-json", false, "Set logging to JSON format.")
	flag.CommandLine.MarkDeprecated("log-json", "Please use --log-encoding=json instead.")
	clientOptions.BindFlags(flag.CommandLine)
	logOptions.BindFlags(flag.CommandLine)
	flag.Parse()

	log := logger.NewLogger(logOptions)
	ctrl.SetLogger(log)

	metricsRecorder := metrics.NewRecorder()
	crtlmetrics.Registry.MustRegister(metricsRecorder.Collectors()...)

	watchNamespace := ""
	if !watchAllNamespaces {
		watchNamespace = os.Getenv("RUNTIME_NAMESPACE")
	}

	restConfig := client.GetConfigOrDie(clientOptions)
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: healthAddr,
		Port:                   9443,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "4ae6d3b3.fluxcd.io",
		Namespace:              watchNamespace,
		Logger:                 ctrl.Log,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	probes.SetupChecks(mgr, setupLog)

	if err = (&controllers.ProviderReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		MetricsRecorder: metricsRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Provider")
		os.Exit(1)
	}
	if err = (&controllers.AlertReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		MetricsRecorder: metricsRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Alert")
		os.Exit(1)
	}
	if err = (&controllers.ReceiverReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		MetricsRecorder: metricsRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Receiver")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	ctx := ctrl.SetupSignalHandler()

	setupLog.Info("starting event server", "addr", eventsAddr)
	eventServer := server.NewEventServer(eventsAddr, log, mgr.GetClient())
	go eventServer.ListenAndServe(ctx.Done())

	setupLog.Info("starting webhook receiver server", "addr", receiverAddr)
	receiverServer := server.NewReceiverServer(receiverAddr, log, mgr.GetClient())
	go receiverServer.ListenAndServe(ctx.Done())

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
