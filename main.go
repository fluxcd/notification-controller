/*
Copyright 2020 The Flux CD contributors.

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
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/notification-controller/api/v1alpha1"
	"github.com/fluxcd/notification-controller/controllers"
	"github.com/fluxcd/notification-controller/internal/server"
	"github.com/fluxcd/pkg/logger"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = v1alpha1.AddToScheme(scheme)
	_ = sourcev1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		eventsAddr           string
		receiverAddr         string
		metricsAddr          string
		enableLeaderElection bool
		concurrent           int
		logLevel             string
		logJSON              bool
	)
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&eventsAddr, "events-addr", ":9090", "The address the event endpoint binds to.")
	flag.StringVar(&receiverAddr, "receiverAddr", ":9292", "The address the webhook receiver endpoint binds to.")

	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&concurrent, "concurrent", 4, "The number of concurrent notification reconciles.")
	flag.StringVar(&logLevel, "log-level", "info", "Set logging level. Can be debug, info or error.")
	flag.BoolVar(&logJSON, "log-json", false, "Set logging to JSON format.")
	flag.Parse()

	zapLogger := logger.NewLogger(logLevel, logJSON)
	ctrl.SetLogger(zapLogger)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "4ae6d3b3.fluxcd.io",
		Namespace:          os.Getenv("RUNTIME_NAMESPACE"),
		Logger:             ctrl.Log,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ProviderReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Provider"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Provider")
		os.Exit(1)
	}
	if err = (&controllers.AlertReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Alert"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Alert")
		os.Exit(1)
	}
	if err = (&controllers.ReceiverReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Receiver"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Receiver")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	stopCh := ctrl.SetupSignalHandler()

	setupLog.Info("starting event server", "addr", eventsAddr)
	eventServer := server.NewEventServer(eventsAddr, zapLogger, mgr.GetClient())
	go eventServer.ListenAndServe(stopCh)

	setupLog.Info("starting webhook receiver server", "addr", receiverAddr)
	receiverServer := server.NewReceiverServer(receiverAddr, zapLogger, mgr.GetClient())
	go receiverServer.ListenAndServe(stopCh)

	setupLog.Info("starting manager")
	if err := mgr.Start(stopCh); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
