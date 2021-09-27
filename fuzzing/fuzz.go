//go:build gofuzz
// +build gofuzz

/*
Copyright 2021 The Flux authors
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	notifyv1 "github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/internal/server"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/events"
	. "github.com/onsi/ginkgo"
	"github.com/sethvargo/go-limiter/memorystore"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	defaultNamespace = "default"
	rcvServer        *httptest.Server
	providerName     = "test-provider"
	provider         notifyv1.Provider
	stopCh           chan struct{}
	req              *http.Request
	initter          sync.Once
	cfgFuzz          *rest.Config
	k8sClientFuzz    client.Client
	testEnvFuzz      *envtest.Environment
	eventMdlwFuzz    middleware.Middleware
	crdPath          []string
	localCRDpath     = []string{filepath.Join("..", "config", "crd", "bases")}

	// OSS-fuzz related variables:
	runningInOssfuzz = false
	alertsCrdURL     = "https://raw.githubusercontent.com/fluxcd/notification-controller/main/config/crd/bases/notification.toolkit.fluxcd.io_alerts.yaml"
	providerCrdURL   = "https://raw.githubusercontent.com/fluxcd/notification-controller/main/config/crd/bases/notification.toolkit.fluxcd.io_providers.yaml"
	receiverCrdUrl   = "https://raw.githubusercontent.com/fluxcd/notification-controller/main/config/crd/bases/notification.toolkit.fluxcd.io_receivers.yaml"
	downloadLink     = "https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-1.19.2-linux-amd64.tar.gz"
	downloadPath     = "/tmp/envtest-bins.tar.gz"
	binariesDir      = "/tmp/test-binaries"
	ossFuzzCrdPath   = []string{filepath.Join(".", "bases")}
)

// Downloads a file to a path
// This is needed to download files when fuzzing in the
// OSS-fuzz environment.
func DownloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// When OSS-fuzz runs the fuzzer a few files need to
// be download during initialization.
// When the fuzzer runs in the OSS-fuzz environment,
// no other files besides the fuzzer itself are available,
// so these have to be produced at runtime.
// downloadFilesForOssFuzz() has the purpose of producing:
// 1: The three binaries that "setup-envtest use" download
// 2: The CRDs
func downloadFilesForOssFuzz() error {
	// Download the three binaries that
	err := DownloadFile(downloadPath, downloadLink)
	if err != nil {
		return err
	}
	err = os.MkdirAll(binariesDir, 0777)
	if err != nil {
		return err
	}
	cmd := exec.Command("tar", "xvf", downloadPath, "-C", binariesDir)
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Download CRDs
	err = os.MkdirAll("bases", 0777)
	if err != nil {
		return err
	}
	err = DownloadFile("./bases/notification.toolkit.fluxcd.io_alerts.yaml", alertsCrdURL)
	if err != nil {
		return err
	}
	err = DownloadFile("./bases/notification.toolkit.fluxcd.io_providers.yaml", providerCrdURL)
	if err != nil {
		return err
	}
	err = DownloadFile("./bases/notification.toolkit.fluxcd.io_receivers.yaml", receiverCrdUrl)
	if err != nil {
		return err
	}
	return nil
}

// createKUBEBUILDER_ASSETS runs "setup-envtest use"
func createKUBEBUILDER_ASSETS() string {
	out, err := exec.Command("setup-envtest", "use").Output()
	if err != nil {
		// If there is an error here, we assume that the fuzzer
		// is running in OSS-fuzz where the binary setup-envtest
		// is not available, so we have to get the test binaries
		// in an alternative way
		runningInOssfuzz = true
		return ""
	}

	// Split the output to get the returned path
	splitString := strings.Split(string(out), " ")
	binPath := strings.TrimSuffix(splitString[len(splitString)-1], "\n")
	if err != nil {
		panic(err)
	}
	return binPath
}

// Custom init func
func initFunc() {
	kubebuilder_assets := createKUBEBUILDER_ASSETS()

	// In case runningInOssfuzz was set to "true" in
	// createKUBEBUILDER_ASSETS(), we set up things
	// now for OSS-fuzz:
	if runningInOssfuzz {
		err := downloadFilesForOssFuzz()
		if err != nil {
			panic(err)
		}
		os.Setenv("KUBEBUILDER_ASSETS", binariesDir+"/kubebuilder/bin")
		crdPath = ossFuzzCrdPath
		runningInOssfuzz = false
	} else {
		os.Setenv("KUBEBUILDER_ASSETS", kubebuilder_assets)
		crdPath = localCRDpath
	}

	logf.SetLogger(
		zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)),
	)

	testEnvFuzz = &envtest.Environment{
		CRDDirectoryPaths: crdPath,
	}

	eventMdlwFuzz = middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix: "gotk_event",
		}),
	})

	// Everything below here is boilerplate set up
	var err error
	cfgFuzz, err = testEnvFuzz.Start()
	if err != nil {
		panic(err)
	}
	if cfgFuzz == nil {
		panic("cfgFuzz is nil but should not be")
	}

	err = notifyv1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	k8sClientFuzz, err = client.New(cfgFuzz, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}
	if k8sClientFuzz == nil {
		panic("k8sClientFuzz is nil but should not be")
	}
	k8sManager, err := ctrl.NewManager(cfgFuzz, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		panic(err)
	}
	if err = (&ProviderReconciler{
		Client: k8sManager.GetClient(),
		Scheme: scheme.Scheme,
	}).SetupWithManager(k8sManager); err != nil {
		panic(err)
	}
	if err = (&AlertReconciler{
		Client: k8sManager.GetClient(),
		Scheme: scheme.Scheme,
	}).SetupWithManager(k8sManager); err != nil {
		panic(err)
	}
	if err = (&ReceiverReconciler{
		Client: k8sManager.GetClient(),
		Scheme: scheme.Scheme,
	}).SetupWithManager(k8sManager); err != nil {
		panic(err)
	}
	time.Sleep(2 * time.Second)
	go func() {
		fmt.Println("Starting k8sManager...")
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		if err != nil {
			panic(err)
		}
	}()
}

// Allows the fuzzer to pick an eventkind
func getEventKind(f *fuzz.ConsumeFuzzer) (string, error) {
	kinds := []string{"GitRepository", "Bucket", "Kustomization",
		"HelmRelease", "HelmChart", "HelmRepository",
		"ImageRepository", "ImagePolicy", "ImageUpdateAutomation"}

	index, err := f.GetInt()
	if err != nil {
		return "", err
	}
	return kinds[index%len(kinds)], nil
}

// Allows the fuzzer to create a slice CrossNamespaceObjectReferences
func createCNORs(f *fuzz.ConsumeFuzzer) ([]notifyv1.CrossNamespaceObjectReference, error) {
	eventsources := make([]notifyv1.CrossNamespaceObjectReference, 0)
	number, err := f.GetInt()
	if err != nil {
		return eventsources, err
	}
	maxReferences := 20
	for i := 0; i < number%maxReferences; i++ {
		cnor := notifyv1.CrossNamespaceObjectReference{}
		err = f.GenerateStruct(&cnor)
		if err != nil {
			return eventsources, err
		}
		kind, err := getEventKind(f)
		if err != nil {
			return eventsources, err
		}
		cnor.Kind = kind
		if cnor.Name == "" {
			return eventsources, errors.New("Name not created")
		}
		if cnor.Namespace == "" {
			return eventsources, errors.New("Namespace not created")
		}
		eventsources = append(eventsources, cnor)
	}
	if len(eventsources) == 0 {
		return eventsources, errors.New("No eventsources created")
	}
	return eventsources, nil
}

// Allows the fuzzer to pick a provider type
func getProviderType(f *fuzz.ConsumeFuzzer) (string, error) {
	providerTypes := []string{"generic",
		"slack",
		"discord",
		"msteams",
		"rocket",
		"github",
		"gitlab",
		"bitbucket",
		"azuredevops",
		"googlechat",
		"webex",
		"sentry",
		"azureeventhub",
		"telegram",
		"lark",
		"matrix"}
	index, err := f.GetInt()
	if err != nil {
		return "", err
	}
	return providerTypes[index%len(providerTypes)], nil
}

// FuzzEventServer implements the fuzzer.
// The target is the event server to which the fuzzer
// sends pseudo-random post-requests.
func Fuzz(data []byte) int {
	initter.Do(initFunc)

	ctx := context.Background()
	f := fuzz.NewConsumer(data)

	// Create object references
	eventsources, err := createCNORs(f)
	if err != nil {
		return 0
	}
	// Get data from an eventsource
	eventSourceIndex, err := f.GetInt()
	if err != nil {
		return 0
	}
	objectReference := eventsources[eventSourceIndex%len(eventsources)]

	// Create a provider ref
	providerRef, err := f.GetStringFrom("abcdefghijklmnopqrstuvwxyz123456789", 50)
	if err != nil {
		return 0
	}

	// Create alert. We wait with instructing the client
	// to create the object later, and only add values for now.
	// This is to postpone the heavy set up stuff (like the
	// httptest server)
	var alert notifyv1.Alert
	err = f.GenerateStruct(&alert)
	if err != nil {
		return 0
	}
	alert.Spec.EventSources = eventsources
	alert.Name = "test-alert"
	alert.Namespace = defaultNamespace
	alert.Spec.ProviderRef.Name = providerRef

	// Create event. This is also created later by the client.
	event := events.Event{}
	err = f.GenerateStruct(&event)
	if err != nil {
		return 0
	}
	event.InvolvedObject.Kind = objectReference.Kind
	event.InvolvedObject.Namespace = objectReference.Namespace
	event.InvolvedObject.Name = objectReference.Name
	buf := &bytes.Buffer{}
	err = json.NewEncoder(buf).Encode(&event)
	if err != nil {
		return 0
	}

	// Create test server
	rcvServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		w.WriteHeader(200)
	}))
	defer func() {
		req = nil
		rcvServer.Close()
	}()

	// Create provider
	provider = notifyv1.Provider{
		Spec: notifyv1.ProviderSpec{
			Address: rcvServer.URL,
		},
	}
	// Set these things to guide the fuzzer a bit inside the reconciler
	provider.Name = providerRef
	provider.Namespace = defaultNamespace
	provider.Spec.SecretRef = nil
	provider.Spec.CertSecretRef = nil
	providerType, err := getProviderType(f)
	if err != nil {
		return 0
	}
	provider.Spec.Type = providerType
	err = k8sClientFuzz.Create(ctx, &provider)
	if err != nil {
		return 0
	}
	defer k8sClientFuzz.Delete(context.Background(), &provider)

	// Instruct the client to create the alert.
	err = k8sClientFuzz.Create(context.Background(), &alert)
	if err != nil {
		return 0
	}
	defer k8sClientFuzz.Delete(context.Background(), &alert)

	store, err := memorystore.New(&memorystore.Config{
		Interval: 5 * time.Minute,
	})
	if err != nil {
		panic(err)
	}

	// the event server won't dispatch to an alert if it has
	// not been marked "ready".
	meta.SetResourceCondition(&alert, meta.ReadyCondition, metav1.ConditionTrue, meta.ReconciliationSucceededReason, "artificially set to ready")
	err = k8sClientFuzz.Status().Update(context.Background(), &alert)
	if err != nil {
		return 0
	}

	// Setup the event server and start it
	eventServer := server.NewEventServer("127.0.0.1:56789", logf.Log, k8sClientFuzz)
	stopCh = make(chan struct{})
	go eventServer.ListenAndServe(stopCh, eventMdlwFuzz, store)

	// Send POST request to event server.
	// An optimization here could be to send multiple requests.
	_, _ = http.Post("http://localhost:56789/", "application/json", buf)
	close(stopCh)
	return 1
}
