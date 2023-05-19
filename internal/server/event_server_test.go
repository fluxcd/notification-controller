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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
	corev1 "k8s.io/api/core/v1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestEventKeyFunc(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup middleware
	store, err := memorystore.New(&memorystore.Config{
		Interval: 10 * time.Minute,
	})
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	middleware, err := httplimit.NewMiddleware(store, eventKeyFunc)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	handler := middleware.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make request
	tests := []struct {
		involvedObject corev1.ObjectReference
		severity       string
		message        string
		rateLimit      bool
		metadata       map[string]string
	}{
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "1",
				Namespace:  "1",
			},
			severity:  eventv1.EventSeverityInfo,
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
			severity:  eventv1.EventSeverityInfo,
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
			severity:  eventv1.EventSeverityError,
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
			severity:  eventv1.EventSeverityInfo,
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
			severity:  eventv1.EventSeverityInfo,
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
			severity:  eventv1.EventSeverityInfo,
			message:   "Health check passed",
			rateLimit: true,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "4",
				Namespace:  "4",
			},
			severity: eventv1.EventSeverityInfo,
			message:  "Health check passed",
			metadata: map[string]string{
				fmt.Sprintf("%s/%s", "kustomize.toolkit.fluxcd.io", eventv1.MetaRevisionKey): "rev1",
			},
			rateLimit: false,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "4",
				Namespace:  "4",
			},
			severity: eventv1.EventSeverityInfo,
			message:  "Health check passed",
			metadata: map[string]string{
				fmt.Sprintf("%s/%s", "kustomize.toolkit.fluxcd.io", eventv1.MetaRevisionKey): "rev1",
			},
			rateLimit: true,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "4",
				Namespace:  "4",
			},
			severity: eventv1.EventSeverityInfo,
			message:  "Health check passed",
			metadata: map[string]string{
				fmt.Sprintf("%s/%s", "kustomize.toolkit.fluxcd.io", eventv1.MetaRevisionKey): "rev2",
			},
			rateLimit: false,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "4",
				Namespace:  "4",
			},
			severity: eventv1.EventSeverityInfo,
			message:  "Health check passed",
			metadata: map[string]string{
				fmt.Sprintf("%s/%s", "kustomize.toolkit.fluxcd.io", eventv1.MetaTokenKey): "token1",
			},
			rateLimit: false,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "4",
				Namespace:  "4",
			},
			severity: eventv1.EventSeverityInfo,
			message:  "Health check passed",
			metadata: map[string]string{
				fmt.Sprintf("%s/%s", "kustomize.toolkit.fluxcd.io", eventv1.MetaTokenKey): "token1",
			},
			rateLimit: true,
		},
		{
			involvedObject: corev1.ObjectReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta1",
				Kind:       "Kustomization",
				Name:       "4",
				Namespace:  "4",
			},
			severity: eventv1.EventSeverityInfo,
			message:  "Health check passed",
			metadata: map[string]string{
				fmt.Sprintf("%s/%s", "kustomize.toolkit.fluxcd.io", eventv1.MetaTokenKey): "token2",
			},
			rateLimit: false,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			event := &eventv1.Event{
				InvolvedObject: tt.involvedObject,
				Severity:       tt.severity,
				Message:        tt.message,
				Metadata:       tt.metadata,
			}
			cleanupMetadata(event)
			eventData, err := json.Marshal(event)
			g.Expect(err).ShouldNot(gomega.HaveOccurred())

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", bytes.NewBuffer(eventData))
			ctxWithEvent := context.WithValue(req.Context(), eventContextKey{}, event)
			reqWithEvent := req.WithContext(ctxWithEvent)
			handler.ServeHTTP(res, reqWithEvent)

			if tt.rateLimit {
				g.Expect(res.Code).Should(gomega.Equal(429))
				g.Expect(res.Header().Get("X-Ratelimit-Remaining")).Should(gomega.Equal("0"))
			} else {
				g.Expect(res.Code).Should(gomega.Equal(200))
			}
		})
	}
}
