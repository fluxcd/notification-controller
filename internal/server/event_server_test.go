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

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
	"github.com/sethvargo/go-limiter/noopstore"
	"github.com/slok/go-http-metrics/middleware"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	notifyv1 "github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/events"
)

func TestEventKeyFunc(t *testing.T) {
	g := NewWithT(t)

	// Setup middleware
	store, err := memorystore.New(&memorystore.Config{
		Interval: 10 * time.Minute,
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	middleware, err := httplimit.NewMiddleware(store, eventKeyFunc)
	g.Expect(err).ShouldNot(HaveOccurred())
	handler := middleware.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make request
	tests := []struct {
		involvedObject corev1.ObjectReference
		severity       string
		message        string
		rateLimit      bool
	}{
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "1",
				Namespace:  "1",
			},
			severity:  events.EventSeverityInfo,
			message:   "Health check passed",
			rateLimit: false,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "1",
				Namespace:  "1",
			},
			severity:  events.EventSeverityInfo,
			message:   "Health check passed",
			rateLimit: true,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "1",
				Namespace:  "1",
			},
			severity:  events.EventSeverityError,
			message:   "Health check timed out for [Deployment 'foo/bar']",
			rateLimit: false,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "2",
				Namespace:  "2",
			},
			severity:  events.EventSeverityInfo,
			message:   "Health check passed",
			rateLimit: false,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "3",
				Namespace:  "3",
			},
			severity:  events.EventSeverityInfo,
			message:   "Health check passed",
			rateLimit: false,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "2",
				Namespace:  "2",
			},
			severity:  events.EventSeverityInfo,
			message:   "Health check passed",
			rateLimit: true,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			event := events.Event{
				InvolvedObject: tt.involvedObject,
				Severity:       tt.severity,
				Message:        tt.message,
			}
			eventData, err := json.Marshal(event)
			g.Expect(err).ShouldNot(HaveOccurred())

			req := httptest.NewRequest("POST", "/", bytes.NewBuffer(eventData))
			g.Expect(err).ShouldNot(HaveOccurred())
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			if tt.rateLimit {
				g.Expect(res.Code).Should(Equal(http.StatusTooManyRequests))
				g.Expect(res.Header().Get("X-Ratelimit-Remaining")).Should(Equal("0"))
			} else {
				g.Expect(res.Code).Should(Equal(http.StatusOK))
			}
		})
	}
}

func TestBlockInsecureHTTP(t *testing.T) {
	g := NewWithT(t)

	var requestsReceived int
	rcvServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsReceived++
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer rcvServer.Close()

	utilruntime.Must(notifyv1.AddToScheme(scheme.Scheme))

	testNamespace := "test-ns"
	providerKey := "provider"
	client := fake.NewFakeClientWithScheme(scheme.Scheme,
		&notifyv1.Provider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      providerKey,
				Namespace: testNamespace,
			},
			Spec: notifyv1.ProviderSpec{
				Type:    "generic",
				Address: rcvServer.URL,
			},
		},
		&notifyv1.Alert{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-alert-name",
				Namespace: testNamespace,
			},
			Spec: notifyv1.AlertSpec{
				ProviderRef: meta.LocalObjectReference{
					Name: providerKey,
				},
				EventSeverity: "info",
				EventSources: []notifyv1.CrossNamespaceObjectReference{
					{
						Kind:      "Bucket",
						Name:      "hyacinth",
						Namespace: testNamespace,
					},
				},
			},
			Status: notifyv1.AlertStatus{
				Conditions: []metav1.Condition{
					{Type: meta.ReadyCondition, Status: metav1.ConditionTrue},
				},
			},
		},
	)

	eventMdlw := middleware.New(middleware.Config{})

	store, err := noopstore.New()
	g.Expect(err).ToNot(HaveOccurred())

	serverEndpoint := "127.0.0.1:56789"
	eventServer := NewEventServer(serverEndpoint, logf.Log, client, true, true)
	stopCh := make(chan struct{})
	go eventServer.ListenAndServe(stopCh, eventMdlw, store)
	defer close(stopCh)

	event := events.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Bucket",
			Name:      "hyacinth",
			Namespace: testNamespace,
		},
		Severity:            "info",
		Timestamp:           metav1.Now(),
		Message:             "well that happened",
		Reason:              "event-happened",
		ReportingController: "source-controller",
	}

	eventServerTests := []struct {
		name          string
		isHttpEnabled bool
		url           string
		wantRequest   int
	}{
		{
			name:          "http scheme is disabled",
			isHttpEnabled: false,
			wantRequest:   0,
		},
		{
			name:          "http scheme is enabled",
			isHttpEnabled: true,
			wantRequest:   1,
		},
	}
	for _, tt := range eventServerTests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			requestsReceived = 0 // reset counter

			// Change the internal state instead of creating a new server.
			eventServer.supportHttpScheme = tt.isHttpEnabled

			buf := &bytes.Buffer{}
			g.Expect(json.NewEncoder(buf).Encode(&event)).To(Succeed())
			res, err := http.Post("http://"+serverEndpoint, "application/json", buf)

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.StatusCode).To(Equal(http.StatusAccepted))

			// Requests happens async, so should the assertion.
			g.Eventually(func() bool {
				return requestsReceived == tt.wantRequest
			}, 5*time.Second).Should(BeTrue())
		})
	}
}
