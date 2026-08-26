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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestNewBitbucketBasic(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucket("kustomization/gitops-system/0c9c2e41", "https://bitbucket.org/foo/bar", "foo:bar", nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(b.Owner).To(Equal("foo"))
	g.Expect(b.Repo).To(Equal("bar"))
	g.Expect(b.CommitStatus).To(Equal("kustomization/gitops-system/0c9c2e41"))
}

func TestNewBitbucketEmptyCommitStatus(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucket("", "https://bitbucket.org/foo/bar", "foo:bar", nil)
	g.Expect(err).To(HaveOccurred())
}

func TestNewBitbucketInvalidUrl(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucket("kustomization/gitops-system/0c9c2e41", "https://bitbucket.org/foo/bar/baz", "foo:bar", nil)
	g.Expect(err).To(HaveOccurred())
}

func TestNewBitbucketInvalidToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucket("kustomization/gitops-system/0c9c2e41", "https://bitbucket.org/foo/bar", "bar", nil)
	g.Expect(err).To(HaveOccurred())
}

func TestBitbucket_Post_UsesCommitStatusAsName(t *testing.T) {
	g := NewWithT(t)

	commitStatus := "custom/status/from-expr"
	var gotName, gotKey string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/statuses/build/"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"Not found"}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/statuses/build"):
			body, err := io.ReadAll(r.Body)
			g.Expect(err).ToNot(HaveOccurred())
			var payload map[string]any
			g.Expect(json.Unmarshal(body, &payload)).To(Succeed())
			gotName, _ = payload["name"].(string)
			gotKey, _ = payload["key"].(string)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	b, err := NewBitbucket(commitStatus, "https://bitbucket.org/foo/bar", "foo:bar", nil)
	g.Expect(err).ToNot(HaveOccurred())

	apiURL, err := url.Parse(ts.URL + "/2.0")
	g.Expect(err).ToNot(HaveOccurred())
	b.Client.SetApiBaseURL(*apiURL)
	b.Client.HttpClient = ts.Client()

	event := eventv1.Event{
		Severity: eventv1.EventSeverityInfo,
		InvolvedObject: corev1.ObjectReference{
			Kind: "Kustomization",
			Name: "gitops-system",
		},
		Metadata: map[string]string{
			eventv1.MetaRevisionKey: "main@sha1:69b59063470310ebbd88a9156325322a124e55a3",
		},
		Reason: "ApplySucceeded",
	}

	err = b.Post(context.Background(), event)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gotName).To(Equal(commitStatus))
	g.Expect(gotKey).To(Equal(sha1String(commitStatus)))

	// Regression guard: Name must not fall back to formatNameAndDescription.
	formattedName, _ := formatNameAndDescription(event)
	g.Expect(gotName).ToNot(Equal(formattedName))
}
