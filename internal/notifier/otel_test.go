/*
Copyright 2025 The Flux authors

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

package notifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

func TestOTEL_Post(t *testing.T) {
	g := NewWithT(t)
	var receivedRequests []*http.Request
	var receivedBodies [][]byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequests = append(receivedRequests, r)
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		receivedBodies = append(receivedBodies, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tests := []struct {
		name  string
		event func() eventv1.Event
	}{
		{
			name: "test event",
			event: func() eventv1.Event {
				e := testEvent()
				// Mocking the data provided by alert.eventMetadata
				e.Metadata["cluster"] = "my-cluster"
				e.Metadata["region"] = "us-east-2"
				e.Metadata["env"] = "prod"
				return e
			},
		},
		{
			name: "event with related object",
			event: func() eventv1.Event {
				// Use a non-source involved object so parent resolution
				// (collectParentUID) is exercised, and provide a RelatedObject
				// whose UID is used directly as the parent regardless of kind.
				e := testEvent()
				e.InvolvedObject = corev1.ObjectReference{
					APIVersion: "helm.toolkit.fluxcd.io/v2",
					Kind:       "HelmRelease",
					Namespace:  "gitops-system",
					Name:       "webapp",
					UID:        "helmrelease-uid",
				}
				e.RelatedObject = &corev1.ObjectReference{
					APIVersion: "source.toolkit.fluxcd.io/v1",
					Kind:       "GitRepository",
					Namespace:  "gitops-system",
					Name:       "webapp",
					UID:        "related-source-uid",
				}
				return e
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alertMetadata := &metav1.ObjectMeta{
				Name:      "test-alert",
				Namespace: "test-namespace",
				UID:       "test-alert-uid",
			}
			ctx := WithAlertMetadata(context.Background(), *alertMetadata)
			// Create mock client with Kustomization scheme
			scheme := runtime.NewScheme()
			mockClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			otelTrace, err := NewOTLPTracer(ctx, mockClient, ts.URL, "", nil, nil, "", "")
			g.Expect(err).ToNot(HaveOccurred())

			event := tt.event()
			err = otelTrace.Post(ctx, event)
			g.Expect(err).ToNot(HaveOccurred())

			// The provider uses a SimpleSpanProcessor, so the span is exported
			// synchronously before Post returns; the latest captured request is
			// this subtest's.
			req := receivedRequests[len(receivedRequests)-1]
			g.Expect(req.Method).To(Equal("POST"))
			g.Expect(req.Header.Get("Content-Type")).To(ContainSubstring("application/x-protobuf"))
			body := receivedBodies[len(receivedBodies)-1]
			g.Expect(body).ToNot(BeEmpty())

			// Validate OTLP content contains expected span data
			bodyStr := string(body)
			g.Expect(bodyStr).To(ContainSubstring(event.InvolvedObject.Name))
			g.Expect(bodyStr).To(ContainSubstring(event.InvolvedObject.Kind))
			g.Expect(bodyStr).To(ContainSubstring(event.InvolvedObject.Namespace))
			// Check for the transformed attributes set via alert.eventMetadata.
			if v, ok := event.Metadata["cluster"]; ok {
				g.Expect(bodyStr).To(ContainSubstring(v))
			}
			if v, ok := event.Metadata["region"]; ok {
				g.Expect(bodyStr).To(ContainSubstring(v))
			}
			if v, ok := event.Metadata["env"]; ok {
				g.Expect(bodyStr).To(ContainSubstring(v))
			}
		})
	}
}
