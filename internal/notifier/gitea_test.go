/*
Copyright 2022 The Flux authors

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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	testproxy "github.com/fluxcd/notification-controller/tests/proxy"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

func TestGiteaProviderContext(t *testing.T) {
	for _, stage := range []string{"version", "user", "list", "create"} {
		t.Run(stage, func(t *testing.T) {
			g := NewWithT(t)
			cancelled := make(chan struct{}, 1)
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					t.Error(err)
					return
				}
				current := "list"
				body := "[]"
				switch {
				case r.URL.Path == "/api/v1/version":
					current, body = "version", `{"version":"1.18.3"}`
				case r.URL.Path == "/api/v1/user":
					current, body = "user", `{"login":"flux"}`
				case r.Method == http.MethodPost:
					current, body = "create", "{}"
				}
				if current == stage {
					select {
					case <-r.Context().Done():
						cancelled <- struct{}{}
					case <-release:
					}
					return
				}
				if _, err := io.WriteString(w, body); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			defer close(release)
			ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				providerType := apiv1.GiteaProvider
				if stage == "user" {
					providerType = apiv1.GiteaPullRequestCommentProvider
				}
				n, err := NewFactory(ctx, WithURL(server.URL+"/foo/bar"), WithToken("token"),
					WithCommitStatus("status"), WithProviderUID("provider")).Notifier(providerType)
				if err == nil && (stage == "list" || stage == "create") {
					event := testEvent()
					event.Metadata[eventv1.MetaRevisionKey] = "main@sha1:69b59063470310ebbd88a9156325322a124e55a3"
					err = n.Post(ctx, event)
				}
				done <- err
			}()
			g.Eventually(done, "2s").Should(Receive(MatchError(ContainSubstring("context deadline exceeded"))))
			g.Eventually(cancelled, "2s").Should(Receive())
		})
	}
}

// newTestHTTPServer returns an HTTP server mimicking parts of Gitea's API so that tests don't
// need to rely on 3rd-party components to be available (like the try.gitea.io server).
func newTestHTTPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(newGiteaStubHandler(t))
}

// newTestHTTPSServer returns an HTTPS server mimicking parts of Gitea's API so that tests don't
// need to rely on 3rd-party components to be available (like the try.gitea.io server).
func newTestHTTPSServer(t *testing.T) *httptest.Server {
	return httptest.NewTLSServer(newGiteaStubHandler(t))
}

func newGiteaStubHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/version":
			fmt.Fprintf(w, `{"version":"1.18.3"}`)
		case "/api/v1/repos/foo/bar/commits/69b59063470310ebbd88a9156325322a124e55a3/statuses":
			fmt.Fprintf(w, "[]")
		case "/api/v1/repos/foo/bar/statuses/69b59063470310ebbd88a9156325322a124e55a3":
			fmt.Fprintf(w, "{}")
		case "/api/v1/repos/foo/bar/commits/8a9156325322a124e55a369b59063470310ebbd8/statuses":
			fmt.Fprintf(w, "[]")
		case "/api/v1/repos/foo/bar/statuses/8a9156325322a124e55a369b59063470310ebbd8":
			fmt.Fprintf(w, "{}")
		default:
			t.Logf("unknown %s request at %s", r.Method, r.URL.Path)
		}
	})
}

func TestNewGiteaBasic(t *testing.T) {
	gomega := NewWithT(t)
	srv := newTestHTTPServer(t)
	defer srv.Close()

	gitea, err := NewGitea("kustomization/gitops-system/0c9c2e41",
		WithGiteaAddress(srv.URL+"/foo/bar"),
		WithGiteaToken("foobar"),
	)
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(gitea.Owner).To(Equal("foo"))
	gomega.Expect(gitea.Repo).To(Equal("bar"))
}

func TestNewGiteaNilContext(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts []GiteaClientOption
	}{
		{name: "explicit nil", opts: []GiteaClientOption{WithGiteaContext(nil)}},
		{name: "unset factory context", opts: (&notifierOptions{}).GiteaClientOptions()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			server := newTestHTTPServer(t)
			defer server.Close()
			opts := append(tt.opts, WithGiteaAddress(server.URL+"/foo/bar"), WithGiteaToken("token"))
			n, err := NewGitea("status", opts...)
			g.Expect(err).ToNot(HaveOccurred())
			event := testEvent()
			event.Metadata[eventv1.MetaRevisionKey] = "main@sha1:69b59063470310ebbd88a9156325322a124e55a3"
			g.Expect(n.Post(t.Context(), event)).To(Succeed())
		})
	}
}

func TestNewGiteaWithCertPool(t *testing.T) {
	gomega := NewWithT(t)
	srv := newTestHTTPSServer(t)
	defer srv.Close()

	certPool := x509.NewCertPool()
	certPool.AddCert(srv.Certificate())
	tlsConfig := &tls.Config{RootCAs: certPool}

	gitea, err := NewGitea("kustomization/gitops-system/0c9c2e41",
		WithGiteaAddress(srv.URL+"/foo/bar"),
		WithGiteaToken("foobar"),
		WithGiteaTLSConfig(tlsConfig),
	)
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(gitea.Owner).To(Equal("foo"))
	gomega.Expect(gitea.Repo).To(Equal("bar"))
}

func TestNewGiteaNoCertificate(t *testing.T) {
	g := NewWithT(t)
	srv := newTestHTTPSServer(t)
	defer srv.Close()

	certPool := x509.NewCertPool()
	tlsConfig := &tls.Config{RootCAs: certPool}

	_, err := NewGitea("kustomization/gitops-system/0c9c2e41",
		WithGiteaAddress(srv.URL+"/foo/bar"),
		WithGiteaToken("foobar"),
		WithGiteaTLSConfig(tlsConfig),
	)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(ContainSubstring("tls: failed to verify certificate: x509: certificate signed by unknown authority")))
}

func TestNewGiteaWithProxyURL(t *testing.T) {
	gomega := NewWithT(t)
	srv := newTestHTTPServer(t)
	defer srv.Close()
	proxyAddr, _ := testproxy.New(t)
	proxyURL := fmt.Sprintf("http://%s", proxyAddr)

	gitea, err := NewGitea("kustomization/gitops-system/0c9c2e41",
		WithGiteaAddress(srv.URL+"/foo/bar"),
		WithGiteaToken("foobar"),
		WithGiteaProxyURL(parseProxyURL(t, proxyURL)),
	)
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(gitea.Owner).To(Equal("foo"))
	gomega.Expect(gitea.Repo).To(Equal("bar"))
}

func TestNewGiteaWithProxyURLAndCertPool(t *testing.T) {
	gomega := NewWithT(t)
	srv := newTestHTTPSServer(t)
	defer srv.Close()

	certPool := x509.NewCertPool()
	certPool.AddCert(srv.Certificate())
	tlsConfig := &tls.Config{RootCAs: certPool}

	proxyAddr, _ := testproxy.New(t)
	proxyURL := fmt.Sprintf("http://%s", proxyAddr)

	gitea, err := NewGitea("kustomization/gitops-system/0c9c2e41",
		WithGiteaAddress(srv.URL+"/foo/bar"),
		WithGiteaToken("foobar"),
		WithGiteaProxyURL(parseProxyURL(t, proxyURL)),
		WithGiteaTLSConfig(tlsConfig),
	)
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(gitea.Owner).To(Equal("foo"))
	gomega.Expect(gitea.Repo).To(Equal("bar"))
}

func TestNewGiteaInvalidUrl(t *testing.T) {
	g := NewWithT(t)
	srv := newTestHTTPServer(t)
	defer srv.Close()

	_, err := NewGitea("kustomization/gitops-system/0c9c2e41",
		WithGiteaAddress(srv.URL+"/foo/bar/baz"),
		WithGiteaToken("foobar"),
	)
	g.Expect(err).To(MatchError(ContainSubstring("invalid repository id")))
}

func TestNewGiteaEmptyToken(t *testing.T) {
	g := NewWithT(t)
	srv := newTestHTTPServer(t)
	defer srv.Close()

	_, err := NewGitea("kustomization/gitops-system/0c9c2e41",
		WithGiteaAddress(srv.URL+"/foo/bar"),
	)
	g.Expect(err).To(MatchError(ContainSubstring("gitea token cannot be empty")))
}

func TestNewGiteaEmptyCommitStatus(t *testing.T) {
	g := NewWithT(t)
	srv := newTestHTTPServer(t)
	defer srv.Close()

	_, err := NewGitea("",
		WithGiteaAddress(srv.URL+"/foo/bar"),
		WithGiteaToken("foobar"),
	)
	g.Expect(err).To(MatchError(ContainSubstring("commit status cannot be empty")))
}

func TestGitea_Post(t *testing.T) {
	gomega := NewWithT(t)
	srv := newTestHTTPServer(t)
	defer srv.Close()

	gitea, err := NewGitea("kustomization/gitops-system/0c9c2e41",
		WithGiteaAddress(srv.URL+"/foo/bar"),
		WithGiteaToken("foobar"),
	)
	gomega.Expect(err).ToNot(HaveOccurred())

	for _, tt := range []struct {
		name  string
		event eventv1.Event
	}{
		{
			name: "revision key",
			event: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind:      "Kustomization",
					Namespace: "flux-system",
					Name:      "podinfo-repo",
				},
				Severity: "info",
				Timestamp: metav1.Time{
					Time: time.Now(),
				},
				Metadata: map[string]string{
					eventv1.MetaRevisionKey: "main@sha1:69b59063470310ebbd88a9156325322a124e55a3",
				},
				Message: "Service/podinfo/podinfo configured",
				Reason:  "",
			},
		},
		{
			name: "origin revision key",
			event: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind:      "Kustomization",
					Namespace: "flux-system",
					Name:      "podinfo-repo",
				},
				Severity: "info",
				Timestamp: metav1.Time{
					Time: time.Now(),
				},
				Metadata: map[string]string{
					eventv1.MetaRevisionKey:       "main@sha1:69b59063470310ebbd88a9156325322a124e55a3",
					eventv1.MetaOriginRevisionKey: "main@sha1:8a9156325322a124e55a369b59063470310ebbd8",
				},
				Message: "Service/podinfo/podinfo configured",
				Reason:  "",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := gitea.Post(context.Background(), tt.event)
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}
