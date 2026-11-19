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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"

	. "github.com/onsi/gomega"
)

func TestNewGitLabBasic(t *testing.T) {
	gomega := NewWithT(t)
	n, err := NewGitLab(t.Context(), &notifierOptions{
		CommitStatus: "kustomization/gitops-system/0c9c2e41",
		URL:          "https://gitlab.com/foo/bar",
		Token:        "foobar",
	})
	gomega.Expect(err).ToNot(HaveOccurred())
	gitlab := n.(*GitLab)
	gomega.Expect(gitlab.Id).To(Equal("foo/bar"))
	gomega.Expect(gitlab.CommitStatus).To(Equal("kustomization/gitops-system/0c9c2e41"))
}

func TestNewGitLabSubgroups(t *testing.T) {
	gomega := NewWithT(t)
	n, err := NewGitLab(t.Context(), &notifierOptions{
		CommitStatus: "kustomization/gitops-system/0c9c2e41",
		URL:          "https://gitlab.com/foo/bar/baz",
		Token:        "foobar",
	})
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(n.(*GitLab).Id).To(Equal("foo/bar/baz"))
}

func TestNewGitLabSelfHosted(t *testing.T) {
	gomega := NewWithT(t)
	n, err := NewGitLab(t.Context(), &notifierOptions{
		CommitStatus: "kustomization/gitops-system/0c9c2e41",
		URL:          "https://example.com/foo/bar",
		Token:        "foo:bar",
	})
	gomega.Expect(err).ToNot(HaveOccurred())
	gitlab := n.(*GitLab)
	gomega.Expect(gitlab.Id).To(Equal("foo/bar"))
	gomega.Expect(gitlab.Client.BaseURL().Host).To(Equal("example.com"))
}

func TestNewGitLabEmptyToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitLab(t.Context(), &notifierOptions{CommitStatus: "kustomization/gitops-system/0c9c2e41", URL: "https://gitlab.com/foo/bar"})
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitLabEmptyCommitStatus(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitLab(t.Context(), &notifierOptions{URL: "https://gitlab.com/foo/bar", Token: "foobar"})
	g.Expect(err).To(HaveOccurred())
}

func TestGitLab_PostCommitStatusDescription(t *testing.T) {
	const sha = "69b59063470310ebbd88a9156325322a124e55a3"
	for _, tt := range []struct {
		name        string
		description string
		existing    string
		want        string
	}{
		{name: "default description", want: "reason"},
		{name: "custom description", description: "Helm upgrade\nfailed", want: "Helm upgrade failed"},
		{name: "custom description differs from existing", description: "Helm upgrade failed", existing: "reason", want: "Helm upgrade failed"},
		{name: "custom description equals existing", description: "Helm upgrade failed", existing: "Helm upgrade failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			created := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					var opts gitlab.SetCommitStatusOptions
					g.Expect(json.NewDecoder(r.Body).Decode(&opts)).To(Succeed())
					created <- *opts.Description
					_, err := io.WriteString(w, "{}")
					g.Expect(err).ToNot(HaveOccurred())
					return
				}
				statuses := []*gitlab.CommitStatus{}
				if tt.existing != "" {
					statuses = append(statuses, &gitlab.CommitStatus{SHA: sha, Name: "test-status", Status: "success", Description: tt.existing})
				}
				g.Expect(json.NewEncoder(w).Encode(statuses)).To(Succeed())
			}))
			defer server.Close()

			n, err := NewGitLab(t.Context(), &notifierOptions{
				CommitStatus:            "test-status",
				CommitStatusDescription: tt.description,
				URL:                     server.URL + "/foo/bar",
				Token:                   "test-token",
			})
			g.Expect(err).ToNot(HaveOccurred())
			event := testEvent()
			event.Metadata[eventv1.MetaRevisionKey] = "main@sha1:" + sha
			g.Expect(n.Post(t.Context(), event)).To(Succeed())
			if tt.want == "" {
				g.Expect(created).ToNot(Receive())
			} else {
				g.Expect(created).To(Receive(Equal(tt.want)))
			}
		})
	}
}
