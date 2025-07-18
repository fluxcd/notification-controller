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
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/event/v1beta1"
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
		event func() v1beta1.Event
	}{
		{
			name: "test event",
			event: func() v1beta1.Event {
				e := testEvent()
				// Mocking the data provided by alert.eventMetadata
				e.Metadata["cluster"] = "my-cluster"
				e.Metadata["region"] = "us-east-2"
				e.Metadata["env"] = "prod"
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

			otelTrace, err := NewOTLPTracer(ctx, ts.URL, "", nil, nil, "", "")
			g.Expect(err).ToNot(HaveOccurred())

			err = otelTrace.Post(ctx, tt.event())
			g.Expect(err).ToNot(HaveOccurred())

			g.Eventually(func() int {
				return len(receivedRequests)
			}, time.Second*5, time.Millisecond*200).Should(BeNumerically(">", 0))

			// Check the request
			g.Expect(receivedRequests).To(HaveLen(1))
			req := receivedRequests[0]
			g.Expect(req.Method).To(Equal("POST"))
			g.Expect(req.Header.Get("Content-Type")).To(ContainSubstring("application/x-protobuf"))
			g.Expect(receivedBodies[0]).ToNot(BeEmpty())

			// Validate OTLP content contains expected span data
			body := string(receivedBodies[0])
			g.Expect(body).To(ContainSubstring(tt.event().InvolvedObject.Name))
			g.Expect(body).To(ContainSubstring(tt.event().InvolvedObject.Kind))
			g.Expect(body).To(ContainSubstring(tt.event().InvolvedObject.Namespace))
			// Check for the actual transformed attributes:
			g.Expect(body).To(ContainSubstring("my-cluster")) // cluster value
			g.Expect(body).To(ContainSubstring("us-east-2"))  // region value
			g.Expect(body).To(ContainSubstring("prod"))       // env value
		})
	}
}
