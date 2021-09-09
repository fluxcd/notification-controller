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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-limiter/memorystore"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/events"

	notifyv1 "github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/internal/server"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var eventMdlw middleware.Middleware

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(
		zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)),
	)

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	eventMdlw = middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix: "gotk_event",
		}),
	})

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = notifyv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("Event handlers", func() {

	var (
		namespace    = "default"
		rcvServer    *httptest.Server
		providerName = "test-provider"
		provider     notifyv1.Provider
		stopCh       chan struct{}
		req          *http.Request
	)

	// This sets up the minimal objects so that we can test the
	// events handling.
	BeforeEach(func() {
		ctx := context.Background()

		// We're not testing the provider, but this is a way to know
		// whether events have been handled.
		rcvServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req = r
			w.WriteHeader(200)
		}))

		provider = notifyv1.Provider{
			Spec: notifyv1.ProviderSpec{
				Type:    "generic",
				Address: rcvServer.URL,
			},
		}
		provider.Name = providerName
		provider.Namespace = namespace
		By("Creating provider")
		Expect(k8sClient.Create(ctx, &provider)).To(Succeed())

		By("Creating and starting event server")
		store, err := memorystore.New(&memorystore.Config{
			Interval: 5 * time.Minute,
		})
		Expect(err).ShouldNot(HaveOccurred())
		// TODO let OS assign port number
		eventServer := server.NewEventServer("127.0.0.1:56789", logf.Log, k8sClient)
		stopCh = make(chan struct{})
		go eventServer.ListenAndServe(stopCh, eventMdlw, store)
	})

	AfterEach(func() {
		req = nil
		rcvServer.Close()
		close(stopCh)
		Expect(k8sClient.Delete(context.Background(), &provider)).To(Succeed())
	})

	// The following test "templates" will create the alert, then
	// serialise the event and post it to the event server. They
	// differ on what's expected to happen to the event.

	var (
		alert notifyv1.Alert
		event events.Event
	)

	JustBeforeEach(func() {
		alert.Name = "test-alert"
		alert.Namespace = namespace
		Expect(k8sClient.Create(context.Background(), &alert)).To(Succeed())
		// the event server won't dispatch to an alert if it has
		// not been marked "ready"
		conditions.MarkTrue(&alert, meta.ReadyCondition, meta.SucceededReason, "artificially set to ready")
		Expect(k8sClient.Status().Update(context.Background(), &alert)).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), &alert)).To(Succeed())
	})

	testSent := func() {
		buf := &bytes.Buffer{}
		Expect(json.NewEncoder(buf).Encode(&event)).To(Succeed())
		res, err := http.Post("http://localhost:56789/", "application/json", buf)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.StatusCode).To(Equal(202)) // event_server responds with 202 Accepted
	}

	testForwarded := func() {
		Eventually(func() bool {
			return req == nil
		}, "2s", "0.1s").Should(BeFalse())
	}

	testFiltered := func() {
		// The event_server does forwarding in a goroutine, after
		// responding to the POST of the event. This makes it
		// difficult to know whether the provider has filtered the
		// event, or just not run the goroutine yet. For now, I'll use
		// a timeout (and Consistently so it can fail early)
		Consistently(func() bool {
			return req == nil
		}, "1s", "0.1s").Should(BeTrue())
	}

	Describe("event forwarding", func() {
		BeforeEach(func() {
			alert = notifyv1.Alert{}
			alert.Spec = notifyv1.AlertSpec{
				ProviderRef: meta.LocalObjectReference{
					Name: providerName,
				},
				EventSeverity: "info",
				EventSources: []notifyv1.CrossNamespaceObjectReference{
					{
						Kind:      "Bucket",
						Name:      "hyacinth",
						Namespace: "default",
					},
				},
			}
			event = events.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind:      "Bucket",
					Name:      "hyacinth",
					Namespace: "default",
				},
				Severity:            "info",
				Timestamp:           metav1.Now(),
				Message:             "well that happened",
				Reason:              "event-happened",
				ReportingController: "source-controller",
			}
		})

		Context("matching by source", func() {
			It("forwards when source is a match", func() {
				testSent()
				testForwarded()
			})
			It("drops event when source Kind does not match", func() {
				event.InvolvedObject.Kind = "GitRepository"
				testSent()
				testFiltered()
			})
			It("drops event when source name does not match", func() {
				event.InvolvedObject.Name = "slop"
				testSent()
				testFiltered()
			})
			It("drops event when source namespace does not match", func() {
				event.InvolvedObject.Namespace = "all-buckets"
				testSent()
				testFiltered()
			})
		})

		Context("filtering by ExclusionList", func() {
			BeforeEach(func() {
				alert.Spec.ExclusionList = []string{
					"doesnotoccur", // not intended to match
					"well",
				}
			})

			It("forwards event that is not matched", func() {
				event.Message = "not excluded"
				testSent()
				testForwarded()
			})

			It("drops event that is matched by exclusion", func() {
				testSent()
				testFiltered()
			})
		})
	})
})
