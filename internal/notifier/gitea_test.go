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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

// newTestServer returns an HTTP server mimicking parts of Gitea's API so that tests don't
// need to rely on 3rd-party components to be available (like the try.gitea.io server).
func newTestServer(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/version":
			fmt.Fprintf(w, `{"version":"1.18.3"}`)
		case "/api/v1/repos/foo/bar/commits/69b59063470310ebbd88a9156325322a124e55a3/statuses":
			fmt.Fprintf(w, "[]")
		case "/api/v1/repos/foo/bar/statuses/69b59063470310ebbd88a9156325322a124e55a3":
			fmt.Fprintf(w, "{}")
		default:
			t.Logf("unknown %s request at %s", r.Method, r.URL.Path)
		}
	}))
	return srv
}

func TestNewGiteaBasic(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	g, err := NewGitea("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", srv.URL+"/foo/bar", "foobar", nil)
	assert.NoError(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.BaseURL, srv.URL)
}

func TestNewGiteaInvalidUrl(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	_, err := NewGitea("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", srv.URL+"/foo/bar/baz", "foobar", nil)
	assert.ErrorContains(t, err, "invalid repository id")
}

func TestNewGiteaEmptyToken(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	_, err := NewGitea("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", srv.URL+"/foo/bar", "", nil)
	assert.ErrorContains(t, err, "gitea token cannot be empty")
}

func TestGitea_Post(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	g, err := NewGitea("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", srv.URL+"/foo/bar", "foobar", nil)
	assert.Nil(t, err)

	event := eventv1.Event{
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
	}
	err = g.Post(context.Background(), event)
	assert.NoError(t, err)
}
