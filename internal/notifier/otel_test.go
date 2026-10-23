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
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestOTEL_Post(t *testing.T) {
	// Queue, batch size and delay environment settings must not drop the
	// single span or export it before ForceFlush reports the export result.
	t.Setenv("OTEL_BSP_MAX_QUEUE_SIZE", "0")
	t.Setenv("OTEL_BSP_MAX_EXPORT_BATCH_SIZE", "1")
	t.Setenv("OTEL_BSP_SCHEDULE_DELAY", "1")

	for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			g := NewWithT(t)
			bodies := make(chan []byte, 1)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				g.Expect(r.Method).To(Equal(http.MethodPost))
				g.Expect(r.Header.Get("Content-Type")).To(ContainSubstring("application/x-protobuf"))
				body, err := io.ReadAll(io.LimitReader(r.Body, maxResponseBodyBytes))
				g.Expect(err).ToNot(HaveOccurred())
				bodies <- body
				w.WriteHeader(status)
			}))
			defer ts.Close()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			ctx = WithAlertMetadata(ctx, metav1.ObjectMeta{
				Name:      "test-alert",
				Namespace: "test-namespace",
				UID:       "test-alert-uid",
			})
			tracer, err := NewOTLPTracer(ctx, ts.URL, nil, nil, nil, "", "")
			g.Expect(err).ToNot(HaveOccurred())
			defer tracer.tracerExporter.Shutdown(context.Background())

			event := testEvent()
			event.Metadata["cluster"] = "my-cluster"
			event.Metadata["region"] = "us-east-2"
			event.Metadata["env"] = "prod"
			err = tracer.Post(ctx, event)
			if status == http.StatusOK {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(MatchError(ContainSubstring("500 Internal Server Error")))
				g.Expect(errors.Unwrap(err)).ToNot(BeNil())
			}

			var body []byte
			g.Expect(bodies).To(Receive(&body))
			g.Expect(string(body)).To(ContainSubstring(event.InvolvedObject.Name))
			g.Expect(string(body)).To(ContainSubstring(event.InvolvedObject.Kind))
			g.Expect(string(body)).To(ContainSubstring(event.InvolvedObject.Namespace))
			g.Expect(string(body)).To(ContainSubstring("my-cluster"))
			g.Expect(string(body)).To(ContainSubstring("us-east-2"))
			g.Expect(string(body)).To(ContainSubstring("prod"))
		})
	}
}

func TestOTEL_PostShutdown(t *testing.T) {
	shutdownErr := errors.New("shutdown failed")
	for _, tt := range []struct {
		name            string
		status          int
		skip            bool
		missingMetadata bool
		cancelOnExport  bool
		shutdownErr     error
		wantErr         string
	}{
		{name: "success", status: http.StatusOK},
		{name: "export failure", status: http.StatusInternalServerError, wantErr: "failed to flush OTEL trace:"},
		{name: "HelmRepository skipped", skip: true},
		{name: "missing metadata", missingMetadata: true, wantErr: "alert metadata not found in context"},
		{name: "canceled export", status: http.StatusOK, cancelOnExport: true, wantErr: "context canceled"},
		{name: "shutdown failure", status: http.StatusOK, shutdownErr: shutdownErr},
		{name: "export failure takes precedence", status: http.StatusInternalServerError, shutdownErr: shutdownErr, wantErr: "failed to flush OTEL trace:"},
		{name: "skipped shutdown failure", skip: true, shutdownErr: shutdownErr},
		{name: "missing metadata takes precedence", missingMetadata: true, shutdownErr: shutdownErr, wantErr: "alert metadata not found in context"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var logs []string
			ctx = log.IntoContext(ctx, funcr.New(func(prefix, args string) {
				logs = append(logs, args)
			}, funcr.Options{Verbosity: 1}))
			var requests atomic.Int32
			var closed atomic.Int32
			ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				_, err := io.Copy(io.Discard, r.Body)
				g.Expect(err).ToNot(HaveOccurred())
				if tt.cancelOnExport {
					cancel()
					<-r.Context().Done()
					return
				}
				w.WriteHeader(tt.status)
			}))
			ts.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateClosed {
					closed.Add(1)
				}
			}
			ts.Start()
			defer ts.Close()

			transport := newTransport(nil, nil)
			client := &otelShutdownClient{
				Client: otlptracehttp.NewClient(
					otlptracehttp.WithEndpointURL(ts.URL),
					otlptracehttp.WithHTTPClient(&http.Client{Transport: transport, Timeout: 10 * time.Second}),
				),
				shutdownErr: tt.shutdownErr,
			}
			exporter, err := otlptrace.New(ctx, client)
			g.Expect(err).ToNot(HaveOccurred())
			defer exporter.Shutdown(context.Background())
			tracer := &OTLPTracer{tracerExporter: exporter, transport: transport}

			if !tt.missingMetadata {
				ctx = WithAlertMetadata(ctx, metav1.ObjectMeta{Name: "alert", Namespace: "default"})
			}
			event := testEvent()
			if tt.skip {
				event.InvolvedObject.Kind = "HelmRepository"
			}
			err = tracer.Post(ctx, event)
			if tt.wantErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErr)))
			}
			if tt.shutdownErr != nil {
				g.Expect(errors.Is(err, shutdownErr)).To(BeFalse())
				g.Expect(logs).To(ContainElement(ContainSubstring("Failed to shut down OTEL tracer")))
				g.Expect(logs).To(ContainElement(ContainSubstring(shutdownErr.Error())))
			}
			if tt.cancelOnExport {
				g.Expect(errors.Is(err, context.Canceled)).To(BeTrue())
			}
			g.Expect(client.stops.Load()).To(Equal(int32(1)))
			if tt.skip || tt.missingMetadata {
				g.Expect(requests.Load()).To(BeZero())
			} else {
				g.Expect(requests.Load()).To(Equal(int32(1)))
				g.Eventually(closed.Load, 5*time.Second).Should(Equal(int32(1)))
			}
		})
	}
}

func TestOTEL_HTTPTransport(t *testing.T) {
	for _, mode := range []string{"proxy", "TLS", "proxy and TLS"} {
		for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
			t.Run(mode+"/"+http.StatusText(status), func(t *testing.T) {
				g := NewWithT(t)
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				ctx = WithAlertMetadata(ctx, metav1.ObjectMeta{Name: "alert", Namespace: "default"})

				var requests, proxyRequests, closed atomic.Int32
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					g.Expect(r.Method).To(Equal(http.MethodPost))
					g.Expect(r.URL.Path).To(Equal("/v1/traces"))
					if mode == "proxy" {
						g.Expect(r.URL.IsAbs()).To(BeTrue())
						g.Expect(r.URL.Host).To(Equal("collector.invalid"))
						g.Expect(r.Header.Get("Proxy-Authorization")).To(Equal("Basic dXNlcjpwYXNz"))
						proxyRequests.Add(1)
					} else {
						g.Expect(r.TLS).ToNot(BeNil())
					}
					_, err := io.Copy(io.Discard, r.Body)
					g.Expect(err).ToNot(HaveOccurred())
					w.WriteHeader(status)
				}))
				server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
					if state == http.StateClosed {
						closed.Add(1)
					}
				}
				var proxyURL *url.URL
				var tlsConfig *tls.Config
				address := "http://collector.invalid"
				if mode == "proxy" {
					server.Start()
					proxyURL = parseProxyURL(t, server.URL)
				} else {
					server.StartTLS()
					address = server.URL
					pool := x509.NewCertPool()
					pool.AddCert(server.Certificate())
					tlsConfig = &tls.Config{RootCAs: pool}
					if mode == "proxy and TLS" {
						proxyHandler := goproxy.NewProxyHttpServer()
						proxyHandler.ConnectDial = func(network, addr string) (net.Conn, error) {
							g.Expect(addr).To(Equal(server.Listener.Addr().String()))
							return net.DialTimeout(network, server.Listener.Addr().String(), time.Second)
						}
						proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							g.Expect(r.Method).To(Equal(http.MethodConnect))
							g.Expect(r.Header.Get("Proxy-Authorization")).To(Equal("Basic dXNlcjpwYXNz"))
							proxyRequests.Add(1)
							proxyHandler.ServeHTTP(w, r)
						}))
						defer proxy.Close()
						proxyURL = parseProxyURL(t, proxy.URL)
					}
				}
				defer server.Close()
				if proxyURL != nil {
					proxyURL.User = url.UserPassword("user", "pass")
				}
				tracer, err := NewOTLPTracer(ctx, address+"/v1/traces", proxyURL, nil, tlsConfig, "", "")
				g.Expect(err).ToNot(HaveOccurred())
				defer tracer.tracerExporter.Shutdown(context.Background())

				err = tracer.Post(ctx, testEvent())
				if status == http.StatusOK {
					g.Expect(err).ToNot(HaveOccurred())
				} else {
					g.Expect(err).To(MatchError(ContainSubstring("500 Internal Server Error")))
				}
				g.Expect(requests.Load()).To(Equal(int32(1)))
				if proxyURL != nil {
					g.Expect(proxyRequests.Load()).To(Equal(int32(1)))
				}
				g.Eventually(closed.Load, 5*time.Second).Should(Equal(int32(1)))
			})
		}
	}
}

type otelShutdownClient struct {
	otlptrace.Client
	shutdownErr error
	stops       atomic.Int32
}

func (c *otelShutdownClient) Stop(ctx context.Context) error {
	c.stops.Add(1)
	return errors.Join(c.Client.Stop(ctx), c.shutdownErr)
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
