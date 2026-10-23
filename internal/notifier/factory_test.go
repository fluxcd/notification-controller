/*
Copyright 2026 The Flux authors

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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elazarl/goproxy"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

func TestFactoryProviderTypeEnum(t *testing.T) {
	g := NewWithT(t)
	data, err := os.ReadFile("../../config/crd/bases/notification.toolkit.fluxcd.io_providers.yaml")
	g.Expect(err).ToNot(HaveOccurred())

	type schema struct {
		Properties map[string]schema `json:"properties"`
		Enum       []string          `json:"enum"`
	}
	var crd struct {
		Spec struct {
			Versions []struct {
				Name   string `json:"name"`
				Schema struct {
					OpenAPIV3Schema schema `json:"openAPIV3Schema"`
				} `json:"schema"`
			} `json:"versions"`
		} `json:"spec"`
	}
	g.Expect(yaml.Unmarshal(data, &crd)).To(Succeed())
	for _, version := range crd.Spec.Versions {
		if version.Name != "v1" {
			continue
		}
		providerTypes := version.Schema.OpenAPIV3Schema.Properties["spec"].Properties["type"].Enum
		g.Expect(providerTypes).ToNot(BeEmpty())

		factoryTypes := make([]string, 0, len(notifiers))
		for providerType := range notifiers {
			factoryTypes = append(factoryTypes, providerType)
		}
		g.Expect(providerTypes).To(ConsistOf(factoryTypes))
		return
	}
	t.Fatal("Provider CRD does not contain a v1 schema")
}

func TestFactoryHTTPTransport(t *testing.T) {
	for _, tt := range []struct {
		provider string
		path     string
		methods  []string
	}{
		{apiv1.GitLabProvider, "/1234", []string{"GET", "POST"}},
		{apiv1.GitLabMergeRequestCommentProvider, "/1234", []string{"GET", "GET", "POST"}},
		{apiv1.BitbucketServerProvider, "/scm/foo/bar.git", []string{"GET", "POST"}},
		{apiv1.SentryProvider, "/1", []string{"POST"}},
		{apiv1.Matrix, "", []string{"PUT"}},
		{apiv1.LarkProvider, "/hook", []string{"POST"}},
		{apiv1.GitHubProvider, "/foo/bar", []string{"GET", "POST"}},
		{apiv1.GitHubDispatchProvider, "/foo/bar", []string{"POST"}},
		{apiv1.GitHubPullRequestCommentProvider, "/foo/bar", []string{"GET", "GET", "POST"}},
		{apiv1.DataDogProvider, "", []string{"POST"}},
		{apiv1.DiscordProvider, "/hook", []string{"POST"}},
		{apiv1.GoogleChatProvider, "/hook", []string{"POST"}},
		{apiv1.TelegramProvider, "/bot-token", []string{"POST"}},
	} {
		for _, mode := range []string{"proxy", "TLS", "proxy and TLS"} {
			t.Run(tt.provider+"/"+mode, func(t *testing.T) {
				g := NewWithT(t)
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()

				methods := make(chan string, 10)
				var proxyRequests atomic.Int32
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					methods <- r.Method
					if mode == "proxy" {
						g.Expect(r.URL.IsAbs()).To(BeTrue())
						g.Expect(r.URL.Host).To(Equal("notification.invalid"))
						g.Expect(r.Header.Get("Proxy-Authorization")).To(Equal("Basic dXNlcjpwYXNz"))
						proxyRequests.Add(1)
					} else {
						g.Expect(r.TLS).ToNot(BeNil())
					}
					w.Header().Set("Content-Type", "application/json")
					body := "{}"
					if r.Method == http.MethodGet {
						switch {
						case strings.HasSuffix(r.URL.Path, "/user"):
							body = `{"login":"flux","username":"flux"}`
						case tt.provider != apiv1.BitbucketServerProvider:
							body = "[]"
						}
					}
					_, err := io.WriteString(w, body)
					g.Expect(err).ToNot(HaveOccurred())
				})

				var server *httptest.Server
				var proxyURL *url.URL
				var tlsConfig *tls.Config
				address := "http://notification.invalid"
				if mode == "proxy" {
					server = httptest.NewServer(handler)
					proxyURL = parseProxyURL(t, server.URL)
				} else {
					server = httptest.NewTLSServer(handler)
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
				address += tt.path
				if tt.provider == apiv1.SentryProvider {
					dsn := parseProxyURL(t, address)
					dsn.User = url.User("public-key")
					address = dsn.String()
				}

				factory := NewFactory(ctx,
					WithURL(address),
					WithProxyURL(proxyURL),
					WithTLSConfig(tlsConfig),
					WithToken("test-token"),
					WithChannel("test-channel"),
					WithCommitStatus("test-status"),
					WithProviderUID("test-provider"),
				)
				n, err := factory.Notifier(tt.provider)
				g.Expect(err).ToNot(HaveOccurred())
				if telegram, ok := n.(*Telegram); ok {
					// Telegram's production API address is fixed.
					telegram.url = address
				}
				if sentry, ok := n.(*Sentry); ok {
					defer sentry.Client.Close()
				}

				event := testEvent()
				event.Metadata[eventv1.MetaRevisionKey] = "main@sha1:69b59063470310ebbd88a9156325322a124e55a3"
				event.Metadata[eventv1.MetaChangeRequestKey] = "1"
				g.Expect(n.Post(ctx, event)).To(Succeed())
				if sentry, ok := n.(*Sentry); ok {
					g.Expect(sentry.Client.Flush(5 * time.Second)).To(BeTrue())
				}

				var gotMethods []string
				for range len(tt.methods) {
					select {
					case method := <-methods:
						gotMethods = append(gotMethods, method)
					case <-ctx.Done():
						t.Fatal("timed out waiting for notifier requests")
					}
				}
				g.Expect(gotMethods).To(Equal(tt.methods))
				g.Expect(methods).To(BeEmpty())
				if proxyURL != nil {
					g.Expect(proxyRequests.Load()).To(BeNumerically(">", 0))
				}
			})
		}
	}
}
