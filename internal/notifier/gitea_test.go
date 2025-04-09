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
	"crypto/x509"
	"fmt"
	testproxy "github.com/fluxcd/notification-controller/tests/proxy"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

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
	srv := newTestHTTPServer(t)
	defer srv.Close()

	g, err := NewGitea("kustomization/gitops-system/0c9c2e41", srv.URL+"/foo/bar", "", "foobar", nil)
	assert.NoError(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.BaseURL, srv.URL)
}

func TestNewGiteaWithCertPool(t *testing.T) {
	srv := newTestHTTPSServer(t)
	defer srv.Close()

	certPool := x509.NewCertPool()
	certPool.AddCert(srv.Certificate())

	g, err := NewGitea("kustomization/gitops-system/0c9c2e41", srv.URL+"/foo/bar", "", "foobar", certPool)
	assert.NoError(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.BaseURL, srv.URL)
}

func TestNewGiteaNoCertificate(t *testing.T) {
	srv := newTestHTTPSServer(t)
	defer srv.Close()

	certPool := x509.NewCertPool()

	_, err := NewGitea("kustomization/gitops-system/0c9c2e41", srv.URL+"/foo/bar", "", "foobar", certPool)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "tls: failed to verify certificate: x509: certificate signed by unknown authority")
}

func TestNewGiteaWithProxyURL(t *testing.T) {
	srv := newTestHTTPServer(t)
	defer srv.Close()
	proxyAddr, _ := testproxy.New(t)
	proxyURL := fmt.Sprintf("http://%s", proxyAddr)

	g, err := NewGitea("kustomization/gitops-system/0c9c2e41", srv.URL+"/foo/bar", proxyURL, "foobar", nil)
	assert.NoError(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.BaseURL, srv.URL)
}

func TestNewGiteaWithProxyURLAndCertPool(t *testing.T) {
	srv := newTestHTTPSServer(t)
	defer srv.Close()

	certPool := x509.NewCertPool()
	certPool.AddCert(srv.Certificate())

	proxyAddr, _ := testproxy.New(t)
	proxyURL := fmt.Sprintf("http://%s", proxyAddr)

	g, err := NewGitea("kustomization/gitops-system/0c9c2e41", srv.URL+"/foo/bar", proxyURL, "foobar", certPool)
	assert.NoError(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.BaseURL, srv.URL)
}

func TestNewGiteaInvalidUrl(t *testing.T) {
	srv := newTestHTTPServer(t)
	defer srv.Close()

	_, err := NewGitea("kustomization/gitops-system/0c9c2e41", srv.URL+"/foo/bar/baz", "", "foobar", nil)
	assert.ErrorContains(t, err, "invalid repository id")
}

func TestNewGiteaInvalidProxyUrl(t *testing.T) {
	_, err := NewGitea("kustomization/gitops-system/0c9c2e41", "/foo/bar", "wrong\nURL", "foobar", nil)
	assert.ErrorContains(t, err, "invalid proxy URL")
}

func TestNewGiteaEmptyToken(t *testing.T) {
	srv := newTestHTTPServer(t)
	defer srv.Close()

	_, err := NewGitea("kustomization/gitops-system/0c9c2e41", srv.URL+"/foo/bar", "", "", nil)
	assert.ErrorContains(t, err, "gitea token cannot be empty")
}

func TestNewGiteaEmptyCommitStatus(t *testing.T) {
	srv := newTestHTTPServer(t)
	defer srv.Close()

	_, err := NewGitea("", srv.URL+"/foo/bar", "", "foobar", nil)
	assert.ErrorContains(t, err, "commit status cannot be empty")
}

func TestGitea_Post(t *testing.T) {
	srv := newTestHTTPServer(t)
	defer srv.Close()

	g, err := NewGitea("kustomization/gitops-system/0c9c2e41", srv.URL+"/foo/bar", "", "foobar", nil)
	assert.Nil(t, err)

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
			err := g.Post(context.Background(), tt.event)
			assert.NoError(t, err)
		})
	}
}
