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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v90/github"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/auth/githubapp"
	"github.com/fluxcd/pkg/ssh"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

func TestNewGitHubPullRequestCommentBasic(t *testing.T) {
	g := NewWithT(t)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/user" {
			user := &github.User{Login: github.String("test-user")}
			json.NewEncoder(w).Encode(user)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	n, err := NewGitHubPullRequestComment(context.Background(), &notifierOptions{
		ProviderUID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		URL:         srv.URL + "/foo/bar",
		Token:       "foobar",
	})
	g.Expect(err).ToNot(HaveOccurred())
	gh := n.(*GitHubPullRequestComment)
	g.Expect(gh.Owner).To(Equal("foo"))
	g.Expect(gh.Repo).To(Equal("bar"))
	g.Expect(gh.ProviderUID).To(Equal("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a"))
	g.Expect(gh.UserLogin).To(Equal("test-user"))
}

func TestNewGitHubPullRequestCommentInvalidUrl(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitHubPullRequestComment(context.Background(), &notifierOptions{
		ProviderUID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		URL:         "https://github.com/foo/bar/baz",
		Token:       "foobar",
	})
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitHubPullRequestCommentEmptyToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitHubPullRequestComment(context.Background(), &notifierOptions{
		ProviderUID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		URL:         "https://github.com/foo/bar",
	})
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitHubPullRequestCommentEmptyProviderUID(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitHubPullRequestComment(context.Background(), &notifierOptions{
		URL:   "https://github.com/foo/bar",
		Token: "foobar",
	})
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitHubPullRequestCommentProvider(t *testing.T) {
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
				g := NewWithT(t)
				w.WriteHeader(http.StatusOK)
				var response []byte
				var err error
				response, err = json.Marshal(&githubapp.AppToken{
					Token:     "access-token",
					Slug:      "app-slug",
					ExpiresAt: expiresAt,
				})
				g.Expect(err).ToNot(HaveOccurred())
				w.Write(response)
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			if len(tt.secretData) > 0 {
				tt.secretData["githubAppBaseURL"] = []byte(srv.URL)
			}
			g := NewWithT(t)
			_, err := NewGitHubPullRequestComment(context.Background(), &notifierOptions{
				ProviderUID:       "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				URL:               "https://github.com/foo/bar",
				ProviderName:      "foo",
				ProviderNamespace: "bar",
				SecretData:        tt.secretData,
			})
			if tt.wantErr != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(Equal(tt.wantErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestGitHubPullRequestCommentPagination(t *testing.T) {
	for _, tt := range []struct {
		name       string
		config     *apiv1.ChangeRequestComment
		matchPage  int
		failPage   int
		wantPages  int
		wantMethod string
		cancelPage int
		writeError string
	}{
		{name: "default searches one page", matchPage: 2, wantPages: 1, wantMethod: http.MethodPost},
		{name: "explicitly disabled", config: &apiv1.ChangeRequestComment{}, matchPage: 2, wantPages: 1, wantMethod: http.MethodPost},
		{name: "find on second page", config: &apiv1.ChangeRequestComment{FetchPages: true}, matchPage: 2, wantPages: 2, wantMethod: http.MethodPatch},
		{name: "stop on first page", config: &apiv1.ChangeRequestComment{FetchPages: true}, matchPage: 1, wantPages: 1, wantMethod: http.MethodPatch},
		{name: "create after last page", config: &apiv1.ChangeRequestComment{FetchPages: true}, wantPages: 3, wantMethod: http.MethodPost},
		{name: "first page error", config: &apiv1.ChangeRequestComment{FetchPages: true}, failPage: 1, wantPages: 1},
		{name: "later page error", config: &apiv1.ChangeRequestComment{FetchPages: true}, failPage: 2, wantPages: 2},
		{name: "cancel on second page", config: &apiv1.ChangeRequestComment{FetchPages: true}, cancelPage: 2, wantPages: 2},
		{name: "create error", config: &apiv1.ChangeRequestComment{FetchPages: true}, wantPages: 3, wantMethod: http.MethodPost, writeError: "failed to create pull request comment"},
		{name: "update error", config: &apiv1.ChangeRequestComment{FetchPages: true}, matchPage: 2, wantPages: 2, wantMethod: http.MethodPatch, writeError: "failed to update pull request comment"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			event := eventv1.Event{
				InvolvedObject: corev1.ObjectReference{Kind: "Kustomization", Namespace: "flux-system", Name: "preview"},
				Metadata:       map[string]string{eventv1.MetaChangeRequestKey: "7"},
				Message:        "deployment failed",
			}
			key := (&changeRequestComment{ProviderUID: "provider", CommentKeyPrefix: "flux-pr-comment-key"}).formatCommentKeyMarker(&event)
			var pages, writes int
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			cancelled := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v3/user" {
					_, _ = fmt.Fprint(w, `{"login":"flux"}`)
					return
				}
				if r.Method == http.MethodGet {
					pages++
					g.Expect(r.URL.Path).To(Equal("/api/v3/repos/foo/bar/issues/7/comments"))
					g.Expect(r.URL.Query().Get("per_page")).To(Equal("100"))
					g.Expect(r.URL.Query().Get("sort")).To(Equal("created"))
					g.Expect(r.URL.Query().Get("direction")).To(Equal("asc"))
					if pages > 1 {
						g.Expect(r.URL.Query().Get("page")).To(Equal(fmt.Sprint(pages)))
					}
					if pages == tt.failPage {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = fmt.Fprint(w, `{"message":"listing failed"}`)
						return
					}
					if pages == tt.cancelPage {
						defer close(cancelled)
						cancel()
						<-r.Context().Done()
						return
					}
					if pages < 3 {
						w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=%d&per_page=100>; rel="next"`, r.Host, r.URL.EscapedPath(), pages+1))
					}
					if pages == tt.matchPage {
						g.Expect(json.NewEncoder(w).Encode([]any{map[string]any{
							"id": 42, "user": map[string]string{"login": "flux"}, "body": key,
						}})).To(Succeed())
					} else {
						// Neither another user's marker nor an unrelated Flux comment matches.
						g.Expect(json.NewEncoder(w).Encode([]any{
							map[string]any{"id": 1, "user": map[string]string{"login": "reviewer"}, "body": key},
							map[string]any{"id": 2, "user": map[string]string{"login": "flux"}, "body": "unrelated"},
						})).To(Succeed())
					}
					return
				}
				writes++
				g.Expect(r.Method).To(Equal(tt.wantMethod))
				if r.Method == http.MethodPatch {
					g.Expect(r.URL.Path).To(Equal("/api/v3/repos/foo/bar/issues/comments/42"))
				} else {
					g.Expect(r.URL.Path).To(Equal("/api/v3/repos/foo/bar/issues/7/comments"))
				}
				var comment struct{ Body string }
				g.Expect(json.NewDecoder(r.Body).Decode(&comment)).To(Succeed())
				g.Expect(comment.Body).To(ContainSubstring(key))
				g.Expect(comment.Body).To(ContainSubstring(event.Message))
				if tt.writeError != "" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = fmt.Fprint(w, `{"message":"write failed"}`)
					return
				}
				_, _ = fmt.Fprint(w, `{}`)
			}))
			t.Cleanup(srv.Close)
			n, err := New(context.Background(), apiv1.GitHubPullRequestCommentProvider, WithURL(srv.URL+"/foo/bar"),
				WithToken("token"), WithProviderUID("provider"), WithChangeRequestComment(tt.config))
			g.Expect(err).NotTo(HaveOccurred())
			err = n.Post(ctx, event)
			if tt.cancelPage != 0 {
				<-cancelled
				g.Expect(err).To(MatchError(ContainSubstring("failed to list pull request comments")))
				g.Expect(err).To(MatchError(ContainSubstring(context.Canceled.Error())))
				g.Expect(writes).To(BeZero())
			} else if tt.failPage != 0 {
				g.Expect(err).To(MatchError(ContainSubstring("failed to list pull request comments")))
				g.Expect(writes).To(BeZero())
			} else if tt.writeError != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.writeError)))
				g.Expect(writes).To(Equal(1))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(writes).To(Equal(1))
			}
			g.Expect(pages).To(Equal(tt.wantPages))
		})
	}
}
