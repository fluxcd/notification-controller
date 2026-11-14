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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

func TestNewGiteaPullRequestCommentBasic(t *testing.T) {
	g := NewWithT(t)

	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/user":
			user := map[string]interface{}{
				"id":       1,
				"login":    "test-user",
				"username": "test-user",
			}
			json.NewEncoder(w).Encode(user)
		case "/api/v1/version":
			fmt.Fprintf(w, `{"version":"1.18.3"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	gc, err := NewGiteaPullRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		WithGiteaAddress(srv.URL+"/foo/bar"),
		WithGiteaToken("foobar"),
	)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gc.Owner).To(Equal("foo"))
	g.Expect(gc.Repo).To(Equal("bar"))
	g.Expect(gc.ProviderUID).To(Equal("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a"))
	g.Expect(gc.Username).To(Equal("test-user"))
}

func TestNewGiteaPullRequestCommentEmptyToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGiteaPullRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		WithGiteaAddress("https://gitea.example.com/foo/bar"),
	)
	g.Expect(err).To(HaveOccurred())
}

func TestNewGiteaPullRequestCommentEmptyProviderUID(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGiteaPullRequestComment("",
		WithGiteaAddress("https://gitea.example.com/foo/bar"),
		WithGiteaToken("foobar"),
	)
	g.Expect(err).To(HaveOccurred())
}

func TestNewGiteaPullRequestCommentInvalidUrl(t *testing.T) {
	g := NewWithT(t)

	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/user":
			user := map[string]interface{}{
				"id":       1,
				"login":    "test-user",
				"username": "test-user",
			}
			json.NewEncoder(w).Encode(user)
		case "/api/v1/version":
			fmt.Fprintf(w, `{"version":"1.18.3"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	_, err := NewGiteaPullRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		WithGiteaAddress(srv.URL+"/foo/bar/baz"),
		WithGiteaToken("foobar"),
	)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid repository id"))
}

func TestGiteaPullRequestCommentPagination(t *testing.T) {
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
				switch r.URL.Path {
				case "/api/v1/user":
					_, _ = fmt.Fprint(w, `{"login":"flux"}`)
					return
				case "/api/v1/version":
					_, _ = fmt.Fprint(w, `{"version":"1.18.3"}`)
					return
				}
				if r.Method == http.MethodGet {
					pages++
					g.Expect(r.URL.Path).To(Equal("/api/v1/repos/foo/bar/issues/7/comments"))
					g.Expect(r.URL.Query().Get("limit")).To(Equal("100"))
					g.Expect(r.URL.Query().Get("page")).To(Equal(fmt.Sprint(pages)))
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
						w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=%d&limit=100>; rel="next"`, r.Host, r.URL.EscapedPath(), pages+1))
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
					g.Expect(r.URL.Path).To(Equal("/api/v1/repos/foo/bar/issues/comments/42"))
				} else {
					g.Expect(r.URL.Path).To(Equal("/api/v1/repos/foo/bar/issues/7/comments"))
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
			n, err := NewFactory(ctx, WithURL(srv.URL+"/foo/bar"),
				WithToken("token"), WithProviderUID("provider"), WithChangeRequestComment(tt.config)).Notifier(apiv1.GiteaPullRequestCommentProvider)
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
