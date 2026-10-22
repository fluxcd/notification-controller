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
	"encoding/base64"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alertMetadata := &metav1.ObjectMeta{
				Name:      "test-alert",
				Namespace: "test-namespace",
				UID:       "test-alert-uid",
			}
			ctx := WithAlertMetadata(context.Background(), *alertMetadata)

			otelTrace, err := NewOTLPTracer(ctx, ts.URL, nil, nil, nil, "", "")
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

func TestOTEL_Authorization(t *testing.T) {
	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	tests := []struct {
		name       string
		headers    map[string]string
		username   string
		password   string
		wantAuth   string
		wantHeader map[string]string
	}{
		{
			name:     "basic auth without headers",
			username: "user",
			password: "pass",
			wantAuth: basic,
		},
		{
			name:     "bearer token without headers",
			password: "token",
			wantAuth: "Bearer token",
		},
		{
			name:       "basic auth with custom headers",
			headers:    map[string]string{"X-Custom": "value"},
			username:   "user",
			password:   "pass",
			wantAuth:   basic,
			wantHeader: map[string]string{"X-Custom": "value"},
		},
		{
			name:     "explicit Authorization header takes precedence",
			headers:  map[string]string{"Authorization": "Custom abc"},
			username: "user",
			password: "pass",
			wantAuth: "Custom abc",
		},
		{
			name:       "no credentials",
			headers:    map[string]string{"X-Custom": "value"},
			wantAuth:   "",
			wantHeader: map[string]string{"X-Custom": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			requests := make(chan http.Header, 1)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Header.Clone()
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			var headersCopy map[string]string
			if tt.headers != nil {
				headersCopy = maps.Clone(tt.headers)
			}

			ctx := WithAlertMetadata(context.Background(), metav1.ObjectMeta{Name: "alert", Namespace: "default"})
			tracer, err := NewOTLPTracer(ctx, ts.URL, nil, tt.headers, nil, tt.username, tt.password)
			g.Expect(err).ToNot(HaveOccurred())
			defer tracer.tracerExporter.Shutdown(context.Background())

			g.Expect(tracer.Post(ctx, testEvent())).To(Succeed())

			var got http.Header
			g.Eventually(requests, 5*time.Second).Should(Receive(&got))
			g.Expect(got.Get("Authorization")).To(Equal(tt.wantAuth))
			for k, v := range tt.wantHeader {
				g.Expect(got.Get(k)).To(Equal(v))
			}
			// The caller's headers map must not be modified.
			g.Expect(tt.headers).To(Equal(headersCopy))
		})
	}
}

func TestOTEL_ProxyCredentials(t *testing.T) {
	g := NewWithT(t)
	type proxyRequest struct {
		url       string
		proxyAuth string
		auth      string
	}
	requests := make(chan proxyRequest, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- proxyRequest{
			url:       r.URL.String(),
			proxyAuth: r.Header.Get("Proxy-Authorization"),
			auth:      r.Header.Get("Authorization"),
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()

	proxyURL := parseProxyURL(t, proxy.URL)
	proxyURL.User = url.UserPassword("proxy-user", "proxy-pass")
	original := proxyURL.String()

	ctx := WithAlertMetadata(context.Background(), metav1.ObjectMeta{Name: "alert", Namespace: "default"})
	tracer, err := NewOTLPTracer(ctx, "http://collector.invalid/v1/traces", proxyURL, nil, nil, "user", "pass")
	g.Expect(err).ToNot(HaveOccurred())
	defer tracer.tracerExporter.Shutdown(context.Background())

	g.Expect(tracer.Post(ctx, testEvent())).To(Succeed())

	var got proxyRequest
	g.Eventually(requests, 5*time.Second).Should(Receive(&got))
	g.Expect(got.url).To(Equal("http://collector.invalid/v1/traces"))
	// The proxy receives the credentials from the proxy secret.
	g.Expect(got.proxyAuth).To(Equal("Basic " + base64.StdEncoding.EncodeToString([]byte("proxy-user:proxy-pass"))))
	// The collector credentials are sent only in the Authorization header.
	g.Expect(got.auth).To(Equal("Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))))
	g.Expect(proxyURL.String()).To(Equal(original))
}
