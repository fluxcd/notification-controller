/*
Copyright 2020, 2021 The Flux authors

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

package controllers

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/testenv"
	"github.com/fluxcd/pkg/ssa"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	notifyv1 "github.com/fluxcd/notification-controller/api/v1beta1"
	// +kubebuilder:scaffold:imports
)

var (
	k8sClient client.Client
	testEnv   *testenv.Environment
	ctx       = ctrl.SetupSignalHandler()
	manager   *ssa.ResourceManager
)

func TestMain(m *testing.M) {
	var err error
	utilruntime.Must(notifyv1.AddToScheme(scheme.Scheme))
	//utilruntime.Must(sourcev1.AddToScheme(scheme.Scheme))

	testEnv = testenv.New(testenv.WithCRDPath(
		filepath.Join("..", "config", "crd", "bases"),
	))

	k8sClient, err = client.New(testEnv.Config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(fmt.Sprintf("failed to create k8s client: %v", err))
	}

	testMetricsH := controller.MustMakeMetrics(testEnv)
	//controllerName := "notification-controller"
	reconciler := AlertReconciler{
		Client:  testEnv,
		Metrics: testMetricsH,
	}
	if err := (reconciler).SetupWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Failed to start AlerReconciler: %v", err))
	}

	if err := (&ProviderReconciler{
		Client: testEnv,
	}).SetupWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Failed to start PRoviderReconciler: %v", err))
	}

	if err := (&ReceiverReconciler{
		Client: testEnv,
	}).SetupWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Failed to start PRoviderReconciler: %v", err))
	}

	go func() {
		fmt.Println("Starting the test environment")
		if err := testEnv.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-testEnv.Manager.Elected()

	restMapper, err := apiutil.NewDynamicRESTMapper(testEnv.Config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create restmapper: %v", restMapper))
	}

	poller := polling.NewStatusPoller(k8sClient, restMapper, nil)
	owner := ssa.Owner{
		Field: "notification-controller",
		Group: "notification-controller",
	}
	manager = ssa.NewResourceManager(k8sClient, poller, owner)

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	fmt.Println("Stopping the event server")

	os.Exit(code)
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func createNamespace(name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	return k8sClient.Create(context.Background(), namespace)
}
