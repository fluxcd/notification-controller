/*
Copyright 2020 The Flux authors

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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/hashicorp/go-retryablehttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

func TestSetRetryBackoff(t *testing.T) {
	g := NewWithT(t)
	defaults := newHTTPClient(&postOptions{})
	g.Expect(defaults.RetryMax).To(Equal(4))
	g.Expect(defaults.RetryWaitMin).To(Equal(2 * time.Second))
	g.Expect(defaults.RetryWaitMax).To(Equal(30 * time.Second))

	t.Run("override", func(t *testing.T) {
		g := NewWithT(t)
		t.Cleanup(SetRetryBackoff(time.Millisecond, 5*time.Millisecond))

		client := newHTTPClient(&postOptions{})
		g.Expect(client.RetryMax).To(Equal(4))
		g.Expect(client.RetryWaitMin).To(Equal(time.Millisecond))
		g.Expect(client.RetryWaitMax).To(Equal(5 * time.Millisecond))
		g.Expect(defaults.RetryWaitMin).To(Equal(2 * time.Second))
		g.Expect(defaults.RetryWaitMax).To(Equal(30 * time.Second))

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		err := postMessage(ctx, server.URL, nil)
		g.Expect(err).To(MatchError(ContainSubstring("unexpected HTTP status 500")))
		g.Expect(attempts.Load()).To(Equal(int32(5)))
	})

	restored := newHTTPClient(&postOptions{})
	g.Expect(restored.RetryMax).To(Equal(defaults.RetryMax))
	g.Expect(restored.RetryWaitMin).To(Equal(defaults.RetryWaitMin))
	g.Expect(restored.RetryWaitMax).To(Equal(defaults.RetryWaitMax))
}

func Test_postMessage(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload = make(map[string]string)
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload["status"]).To(Equal("success"))
	}))
	defer ts.Close()
	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"})
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_postMessage_accepts2xxStatusCodes(t *testing.T) {
	for _, code := range []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNoContent,
	} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			g := NewWithT(t)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			defer ts.Close()

			err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"})
			g.Expect(err).ToNot(HaveOccurred())
		})
	}

	t.Run(http.StatusText(http.StatusBadRequest), func(t *testing.T) {
		g := NewWithT(t)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("nope"))
		}))
		defer ts.Close()

		err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("request failed with status code 400"))
	})
}

func Test_postMessage_timeout(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := postMessage(ctx, ts.URL, map[string]string{"status": "success"})
	g.Expect(err).To(HaveOccurred())
}

func Test_postSelfSignedCert(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload = make(map[string]string)
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload["status"]).To(Equal("success"))
	}))
	defer ts.Close()

	cert, err := x509.ParseCertificate(ts.TLS.Certificates[0].Certificate[0])
	g.Expect(err).ToNot(HaveOccurred())
	certpool := x509.NewCertPool()
	certpool.AddCert(cert)
	tlsConfig := &tls.Config{RootCAs: certpool}
	err = postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withTLSConfig(tlsConfig))
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_postMessage_contentType(t *testing.T) {
	const contentType = "application/x-www-form-urlencoded"

	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.Header.Get("Content-Type")).To(Equal(contentType))

		err := r.ParseForm()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(r.Form.Get("status")).To(Equal("success"))
	}))
	defer ts.Close()

	err := postMessage(context.Background(), ts.URL, []byte("status=success"), withContentType(contentType))
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_postMessage_contentTypeRequiresBytes(t *testing.T) {
	for _, payload := range []any{"status=success", map[string]string{"status": "success"}, nil} {
		g := NewWithT(t)
		err := postMessage(t.Context(), "http://notification.invalid", payload,
			withContentType("application/x-www-form-urlencoded"))
		g.Expect(err).To(MatchError(ContainSubstring("notification payload must be []byte")))
	}
}

func TestPostMessageLimitsErrorBody(t *testing.T) {
	g := NewWithT(t)
	server, written := newLargeResponseServer(http.StatusBadRequest, "", "")
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := postMessage(ctx, server.URL, map[string]string{"status": "success"})
	const prefix = "request failed: request failed with status code 400: "
	g.Expect(err).To(MatchError(prefix + strings.Repeat("x", 4*1024) + "... (truncated)"))
	g.Eventually(written).Should(Receive(BeNumerically("<", 100*1024*1024)))
}

func TestPostMessageErrorBodyLimitBoundary(t *testing.T) {
	for _, size := range []int{4*1024 - 1, 4 * 1024, 4*1024 + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			g := NewWithT(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				if _, err := io.WriteString(w, strings.Repeat("x", size)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			err := postMessage(t.Context(), server.URL, nil)
			want := "request failed: request failed with status code 400: " + strings.Repeat("x", min(size, 4*1024))
			if size > 4*1024 {
				want += "... (truncated)"
			}
			g.Expect(err).To(MatchError(want))
		})
	}
}

// newLargeResponseServer streams a 100 MiB body without allocating it in full.
func newLargeResponseServer(status int, prefix, suffix string) (*httptest.Server, <-chan int64) {
	written := make(chan int64, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var total int64
		defer func() { written <- total }()
		w.WriteHeader(status)
		n, err := io.WriteString(w, prefix)
		total += int64(n)
		if err != nil {
			return
		}
		chunk := strings.Repeat("x", 32*1024)
		for i := 0; i < 100*1024*1024/len(chunk); i++ {
			n, err := io.WriteString(w, chunk)
			total += int64(n)
			if err != nil {
				return
			}
		}
		n, err = io.WriteString(w, suffix)
		total += int64(n)
		if err != nil {
			return
		}
	}))
	return server, written
}

func Test_postMessage_basicAuth(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		g.Expect(ok).To(BeTrue())
		g.Expect(username).To(Equal("user"))
		g.Expect(password).To(Equal("pass"))

		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload = make(map[string]string)
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(payload["status"]).To(Equal("success"))
	}))
	defer ts.Close()

	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withBasicAuth("user", "pass"))
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_postMessage_requestModifier(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		g.Expect(r.Header.Get("Authorization")).To(Equal("Bearer token"))
	}))
	defer ts.Close()

	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withRequestModifier(func(req *retryablehttp.Request) {
		req.Header.Set("Authorization", "Bearer token")
	}))
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_postMessage_responseValidator(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default response validator determines success, but the custom validator below will determine failure .
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("error: bad request"))
	}))
	defer ts.Close()

	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"})
	g.Expect(err).ToNot(HaveOccurred())

	err = postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withResponseValidator(func(resp *http.Response) error {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if strings.HasPrefix(string(body), "error:") {
			return errors.New(string(body))
		}
		return nil
	}))
	g.Expect(err).To(MatchError(ContainSubstring("request failed: error: bad request")))
}

func testEvent() eventv1.Event {
	return eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "GitRepository",
			Namespace: "gitops-system",
			Name:      "webapp",
		},
		Severity:  "info",
		Timestamp: metav1.Now(),
		Message:   "message",
		Reason:    "reason",
		Metadata: map[string]string{
			"test": "metadata",
		},
		ReportingController: "source-controller",
		ReportingInstance:   "source-controller-xyz",
	}
}

func TestPostMessageWithParsedProxy(t *testing.T) {
	g := NewWithT(t)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.String()).To(Equal("http://notification.invalid/events"))
		g.Expect(r.Header.Get("Proxy-Authorization")).To(Equal("Basic dXNlcjpwYXNz"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	g.Expect(err).ToNot(HaveOccurred())
	proxyURL.User = url.UserPassword("user", "pass")
	g.Expect(postMessage(context.Background(), "http://notification.invalid/events", map[string]string{"message": "test"}, withProxy(proxyURL))).To(Succeed())
}

func parseProxyURL(t testing.TB, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestNewTransport(t *testing.T) {
	// ProxyFromEnvironment caches the environment on first use, so isolate this test.
	if os.Getenv("NOTIFIER_TEST_PROXY_ENV") == "" {
		t.Setenv("HTTP_PROXY", "http://http-proxy.invalid:8080")
		t.Setenv("HTTPS_PROXY", "http://https-proxy.invalid:8080")
		t.Setenv("NO_PROXY", "bypass.invalid")
		t.Setenv("REQUEST_METHOD", "")
		cmd := exec.Command(os.Args[0], "-test.run=^TestNewTransport$")
		cmd.Env = append(os.Environ(), "NOTIFIER_TEST_PROXY_ENV=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated transport test failed: %v\n%s", err, output)
		}
		return
	}

	defaults := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig := &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12}
	explicitProxy := parseProxyURL(t, "http://explicit-proxy.invalid:8080")
	for _, tt := range []struct {
		name      string
		proxy     *url.URL
		tlsConfig *tls.Config
	}{
		{name: "defaults"},
		{name: "TLS with environment proxy", tlsConfig: tlsConfig},
		{name: "explicit proxy", proxy: explicitProxy},
		{name: "explicit proxy and TLS", proxy: explicitProxy, tlsConfig: tlsConfig},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tr := newTransport(tt.proxy, tt.tlsConfig)
			defer tr.CloseIdleConnections()
			g.Expect(tr).ToNot(BeIdenticalTo(http.DefaultTransport))
			g.Expect(tr.DialContext).ToNot(BeNil())
			g.Expect(tr.TLSHandshakeTimeout).To(Equal(defaults.TLSHandshakeTimeout))
			g.Expect(tr.IdleConnTimeout).To(Equal(defaults.IdleConnTimeout))
			g.Expect(tr.MaxIdleConns).To(Equal(defaults.MaxIdleConns))
			g.Expect(tr.ExpectContinueTimeout).To(Equal(defaults.ExpectContinueTimeout))
			g.Expect(tr.ForceAttemptHTTP2).To(Equal(defaults.ForceAttemptHTTP2))
			if tt.tlsConfig != nil {
				g.Expect(tr.TLSClientConfig).ToNot(BeIdenticalTo(tt.tlsConfig))
				g.Expect(tr.TLSClientConfig).To(Equal(tt.tlsConfig))
			} else {
				g.Expect(tr.TLSClientConfig).To(Equal(defaults.TLSClientConfig))
			}

			for _, scheme := range []string{"http", "https"} {
				req, err := http.NewRequest(http.MethodGet, scheme+"://notification.invalid", nil)
				g.Expect(err).ToNot(HaveOccurred())
				proxy, err := tr.Proxy(req)
				g.Expect(err).ToNot(HaveOccurred())
				if tt.proxy != nil {
					g.Expect(proxy).To(Equal(explicitProxy))
				} else {
					g.Expect(proxy.String()).To(Equal("http://" + scheme + "-proxy.invalid:8080"))
				}

				req.URL.Host = "bypass.invalid"
				proxy, err = tr.Proxy(req)
				g.Expect(err).ToNot(HaveOccurred())
				if tt.proxy != nil {
					g.Expect(proxy).To(Equal(explicitProxy))
				} else {
					g.Expect(proxy).To(BeNil())
				}
			}
		})
	}

	g := NewWithT(t)
	g.Expect(http.DefaultTransport.(*http.Transport).TLSClientConfig).To(Equal(defaults.TLSClientConfig))
	g.Expect(reflect.ValueOf(http.DefaultTransport.(*http.Transport).Proxy).Pointer()).
		To(Equal(reflect.ValueOf(http.ProxyFromEnvironment).Pointer()))
}

func TestNewTransportTLSConfigIsolation(t *testing.T) {
	g := NewWithT(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	tlsConfig := &tls.Config{RootCAs: pool}
	for range 2 {
		transport := newTransport(nil, tlsConfig)
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
		g.Expect(err).ToNot(HaveOccurred())
		resp, err := client.Do(req)
		g.Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		g.Expect(resp.ProtoMajor).To(Equal(2))
		g.Expect(transport.TLSClientConfig.NextProtos).To(ContainElement("h2"))
		g.Expect(tlsConfig.NextProtos).To(BeNil())
	}
}
