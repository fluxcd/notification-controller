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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/auth/githubapp"
	"github.com/fluxcd/pkg/ssh"

	"github.com/google/go-github/v64/github"
	. "github.com/onsi/gomega"
)

func TestNewGitHubBasic(t *testing.T) {
	gm := NewWithT(t)
	g, err := NewGitHub(context.Background(), "kustomization/gitops-system/0c9c2e41",
		WithGitHubAddress("https://github.com/foo/bar"),
		WithGitHubToken("foobar"),
	)
	gm.Expect(err).ToNot(HaveOccurred())
	gm.Expect(g.Owner).To(Equal("foo"))
	gm.Expect(g.Repo).To(Equal("bar"))
	gm.Expect(g.Client.BaseURL.Host).To(Equal("api.github.com"))
	gm.Expect(g.CommitStatus).To(Equal("kustomization/gitops-system/0c9c2e41"))
}

func TestNewEmterpriseGitHubBasic(t *testing.T) {
	gm := NewWithT(t)
	g, err := NewGitHub(context.Background(), "kustomization/gitops-system/0c9c2e41",
		WithGitHubAddress("https://foobar.com/foo/bar"),
		WithGitHubToken("foobar"),
	)
	gm.Expect(err).ToNot(HaveOccurred())
	gm.Expect(g.Owner).To(Equal("foo"))
	gm.Expect(g.Repo).To(Equal("bar"))
	gm.Expect(g.Client.BaseURL.Host).To(Equal("foobar.com"))
	gm.Expect(g.CommitStatus).To(Equal("kustomization/gitops-system/0c9c2e41"))
}

func TestNewGitHubInvalidUrl(t *testing.T) {
	gm := NewWithT(t)
	_, err := NewGitHub(context.Background(), "kustomization/gitops-system/0c9c2e41",
		WithGitHubAddress("https://github.com/foo/bar/baz"),
		WithGitHubToken("foobar"),
	)
	gm.Expect(err).To(HaveOccurred())
}

func TestNewGitHubEmptyToken(t *testing.T) {
	gm := NewWithT(t)
	_, err := NewGitHub(context.Background(), "kustomization/gitops-system/0c9c2e41",
		WithGitHubAddress("https://github.com/foo/bar"),
	)
	gm.Expect(err).To(HaveOccurred())
}

func TestNewGitHubEmptyCommitStatus(t *testing.T) {
	gm := NewWithT(t)
	_, err := NewGitHub(context.Background(), "",
		WithGitHubAddress("https://github.com/foo/bar"),
		WithGitHubToken("foobar"),
	)
	gm.Expect(err).To(HaveOccurred())
}

func TestNewGithubProvider(t *testing.T) {
	gm := NewWithT(t)
	appID := "123"
	installationID := "456"
	kp, _ := ssh.GenerateKeyPair(ssh.RSA_4096)
	expiresAt := time.Now().UTC().Add(time.Hour)

	for _, tt := range []struct {
		name       string
		secretData map[string][]byte
		wantErr    error
	}{
		{
			name:    "nil provider, no token",
			wantErr: errors.New("github token or github app details must be specified"),
		},
		{
			name:       "provider with no github options",
			secretData: map[string][]byte{},
			wantErr:    errors.New("github token or github app details must be specified"),
		},
		{
			name: "provider with missing app ID in options ",
			secretData: map[string][]byte{
				"githubAppInstallationID": []byte(installationID),
				"githubAppPrivateKey":     kp.PrivateKey,
			},
			wantErr: errors.New("github token or github app details must be specified"),
		},
		{
			name: "provider with missing app installation ID in options ",
			secretData: map[string][]byte{
				"githubAppID":         []byte(appID),
				"githubAppPrivateKey": kp.PrivateKey,
			},
			wantErr: errors.New("app installation owner or ID must be provided to use github app authentication"),
		},
		{
			name: "provider with missing app private key in options ",
			secretData: map[string][]byte{
				"githubAppID":             []byte(appID),
				"githubAppInstallationID": []byte(installationID),
			},
			wantErr: errors.New("private key must be provided to use github app authentication"),
		},
		{
			name: "provider with complete app authentication information",
			secretData: map[string][]byte{
				"githubAppID":             []byte(appID),
				"githubAppInstallationID": []byte(installationID),
				"githubAppPrivateKey":     kp.PrivateKey,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				var response []byte
				var err error
				response, err = json.Marshal(&githubapp.AppToken{Token: "access-token", ExpiresAt: expiresAt})
				gm.Expect(err).ToNot(HaveOccurred())
				w.Write(response)
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			if len(tt.secretData) > 0 {
				tt.secretData["githubAppBaseURL"] = []byte(srv.URL)
			}
			_, err := NewGitHub(context.Background(), "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				WithGitHubAddress("https://github.com/foo/bar"),
				WithGitHubProvider("foo", "bar"),
				WithGitHubSecretData(tt.secretData),
			)
			if tt.wantErr != nil {
				gm.Expect(err).To(HaveOccurred())
				gm.Expect(err).To(Equal(tt.wantErr))
			} else {
				gm.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestDuplicateGithubStatus(t *testing.T) {
	gm := NewWithT(t)

	var tests = []struct {
		ss  []*github.RepoStatus
		s   *github.RepoStatus
		dup bool
	}{
		{[]*github.RepoStatus{ghStatus("success", "foo", "bar")}, ghStatus("success", "foo", "bar"), true},
		{[]*github.RepoStatus{ghStatus("success", "foo", "bar")}, ghStatus("failure", "foo", "bar"), false},
		{[]*github.RepoStatus{ghStatus("success", "foo", "bar")}, ghStatus("success", "baz", "bar"), false},
		{[]*github.RepoStatus{ghStatus("success", "foo", "bar")}, ghStatus("success", "foo", "baz"), false},
		{[]*github.RepoStatus{ghStatus("success", "baz", "bar"), ghStatus("success", "foo", "bar")}, ghStatus("success", "foo", "bar"), true},
	}

	for _, test := range tests {
		gm.Expect(duplicateGithubStatus(test.ss, test.s)).To(Equal(test.dup))
	}
}

func TestGitHubPostDuplicateOnLaterPage(t *testing.T) {
	const sha = "8f1e9a2b3c4d5e6f708192a3b4c5d6e7f8091a2b"

	for _, tt := range []struct {
		name string
		// latest status of the notifier's context, served on the second page
		// of the combined status after 100 other contexts; nil when absent.
		latest    *github.RepoStatus
		wantPosts int
	}{
		{
			name:      "same state and description",
			latest:    ghStatus("success", "flux/ks", "reconciliation succeeded"),
			wantPosts: 0,
		},
		{
			name:      "different description",
			latest:    ghStatus("success", "flux/ks", "health check failed"),
			wantPosts: 1,
		},
		{
			name:      "different state",
			latest:    ghStatus("failure", "flux/ks", "reconciliation succeeded"),
			wantPosts: 1,
		},
		{
			name:      "context absent",
			wantPosts: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			posts := 0
			mux := http.NewServeMux()
			mux.HandleFunc("GET /repos/foo/bar/commits/"+sha+"/status", func(w http.ResponseWriter, r *http.Request) {
				var statuses []*github.RepoStatus
				if p := r.URL.Query().Get("page"); p == "" || p == "1" {
					for i := range 100 {
						statuses = append(statuses, ghStatus("success", fmt.Sprintf("flux/other-%d", i), "reconciliation succeeded"))
					}
					w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=2>; rel="next"`, r.Host, r.URL.Path))
				} else if tt.latest != nil {
					statuses = []*github.RepoStatus{tt.latest}
				}
				g.Expect(json.NewEncoder(w).Encode(&github.CombinedStatus{Statuses: statuses})).To(Succeed())
			})
			// Every status ever posted on the commit, newest first: the notifier's
			// latest status sits behind 60 newer ones from other contexts.
			mux.HandleFunc("GET /repos/foo/bar/commits/"+sha+"/statuses", func(w http.ResponseWriter, r *http.Request) {
				var statuses []*github.RepoStatus
				for i := range 60 {
					statuses = append(statuses, ghStatus("success", fmt.Sprintf("flux/other-%d", i), "reconciliation succeeded"))
				}
				if tt.latest != nil {
					statuses = append(statuses, tt.latest)
				}
				if perPage := r.URL.Query().Get("per_page"); perPage == "50" {
					statuses = statuses[:50]
				}
				g.Expect(json.NewEncoder(w).Encode(statuses)).To(Succeed())
			})
			mux.HandleFunc("POST /repos/foo/bar/statuses/"+sha, func(w http.ResponseWriter, r *http.Request) {
				posts++
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("{}"))
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			client := github.NewClient(nil)
			baseURL, err := url.Parse(srv.URL + "/")
			g.Expect(err).ToNot(HaveOccurred())
			client.BaseURL = baseURL

			n := &GitHub{Owner: "foo", Repo: "bar", CommitStatus: "flux/ks", Client: client}
			event := testEvent()
			event.Reason = "ReconciliationSucceeded"
			event.Metadata = map[string]string{eventv1.MetaRevisionKey: "main@sha1:" + sha}

			g.Expect(n.Post(context.Background(), event)).To(Succeed())
			g.Expect(posts).To(Equal(tt.wantPosts))
		})
	}
}

func ghStatus(state string, context string, description string) *github.RepoStatus {
	return &github.RepoStatus{
		State:       &state,
		Context:     &context,
		Description: &description,
	}
}
