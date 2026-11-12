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

func TestNewGitLabMergeRequestCommentBasic(t *testing.T) {
	g := NewWithT(t)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/user" {
			user := map[string]interface{}{
				"id":       1,
				"username": "test-user",
			}
			json.NewEncoder(w).Encode(user)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	gl, err := NewGitLabMergeRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", srv.URL+"/foo/bar", nil, "foobar", nil, false)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gl.ProjectID).To(Equal("foo/bar"))
	g.Expect(gl.ProviderUID).To(Equal("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a"))
	g.Expect(gl.Username).To(Equal("test-user"))
	g.Expect(gl.FetchPages).To(BeFalse())

	gl, err = NewGitLabMergeRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", srv.URL+"/foo/bar", nil, "foobar", nil, true)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gl.FetchPages).To(BeTrue())
}

func TestNewGitLabMergeRequestCommentSubgroups(t *testing.T) {
	g := NewWithT(t)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/user" {
			user := map[string]interface{}{
				"id":       1,
				"username": "test-user",
			}
			json.NewEncoder(w).Encode(user)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	gl, err := NewGitLabMergeRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", srv.URL+"/foo/bar/baz", nil, "foobar", nil, false)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gl.ProjectID).To(Equal("foo/bar/baz"))
}

func TestNewGitLabMergeRequestCommentEmptyToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitLabMergeRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "https://gitlab.com/foo/bar", nil, "", nil, false)
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitLabMergeRequestCommentEmptyProviderUID(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitLabMergeRequestComment("", "https://gitlab.com/foo/bar", nil, "foobar", nil, false)
	g.Expect(err).To(HaveOccurred())
}

func TestGitLabMergeRequestCommentPagination(t *testing.T) {
	for _, tt := range []struct {
		name       string
		config     *apiv1.ChangeRequestComment
		matchPage  int
		failPage   int
		wantPages  int
		wantMethod string
		linkOnly   bool
		emptyLast  bool
		cancelPage int
		writeError string
	}{
		{name: "default searches one page", matchPage: 2, wantPages: 1, wantMethod: http.MethodPost},
		{name: "explicitly disabled", config: &apiv1.ChangeRequestComment{}, matchPage: 2, wantPages: 1, wantMethod: http.MethodPost},
		{name: "follow link header", config: &apiv1.ChangeRequestComment{FetchPages: true}, linkOnly: true, matchPage: 2, wantPages: 2, wantMethod: http.MethodPut},
		{name: "exhaust link pages", config: &apiv1.ChangeRequestComment{FetchPages: true}, linkOnly: true, wantPages: 3, wantMethod: http.MethodPost},
		{name: "ignore link by default", linkOnly: true, matchPage: 2, wantPages: 1, wantMethod: http.MethodPost},
		{name: "find on second page", config: &apiv1.ChangeRequestComment{FetchPages: true}, matchPage: 2, wantPages: 2, wantMethod: http.MethodPut},
		{name: "stop on first page", config: &apiv1.ChangeRequestComment{FetchPages: true}, matchPage: 1, wantPages: 1, wantMethod: http.MethodPut},
		{name: "create after last page", config: &apiv1.ChangeRequestComment{FetchPages: true}, wantPages: 3, wantMethod: http.MethodPost},
		{name: "first page error", config: &apiv1.ChangeRequestComment{FetchPages: true}, failPage: 1, wantPages: 1},
		{name: "later page error", config: &apiv1.ChangeRequestComment{FetchPages: true}, failPage: 2, wantPages: 2},
		{name: "empty final page", config: &apiv1.ChangeRequestComment{FetchPages: true}, emptyLast: true, wantPages: 3, wantMethod: http.MethodPost},
		{name: "cancel on second page", config: &apiv1.ChangeRequestComment{FetchPages: true}, cancelPage: 2, wantPages: 2},
		{name: "create error", config: &apiv1.ChangeRequestComment{FetchPages: true}, wantPages: 3, wantMethod: http.MethodPost, writeError: "failed to create merge request note"},
		{name: "update error", config: &apiv1.ChangeRequestComment{FetchPages: true}, matchPage: 2, wantPages: 2, wantMethod: http.MethodPut, writeError: "failed to update merge request note"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			event := eventv1.Event{
				InvolvedObject: corev1.ObjectReference{Kind: "Kustomization", Namespace: "flux-system", Name: "preview"},
				Metadata:       map[string]string{eventv1.MetaChangeRequestKey: "7"},
				Message:        "deployment failed",
			}
			key := (&changeRequestComment{ProviderUID: "provider", CommentKeyPrefix: "flux-mr-comment-key"}).formatCommentKeyMarker(&event)
			var pages, writes int
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			cancelled := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v4/user" {
					_, _ = fmt.Fprint(w, `{"username":"flux"}`)
					return
				}
				if r.Method == http.MethodGet {
					pages++
					g.Expect(r.URL.Path).To(Equal("/api/v4/projects/foo/bar/merge_requests/7/notes"))
					g.Expect(r.URL.Query().Get("per_page")).To(Equal("100"))
					g.Expect(r.URL.Query().Get("sort")).To(Equal("asc"))
					g.Expect(r.URL.Query().Get("order_by")).To(Equal("created_at"))
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
						if tt.linkOnly {
							w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=%d&per_page=100>; rel="next"`, r.Host, r.URL.EscapedPath(), pages+1))
						} else {
							w.Header().Set("X-Next-Page", fmt.Sprint(pages+1))
						}
					}
					if tt.emptyLast && pages == 3 {
						_, _ = fmt.Fprint(w, `[]`)
					} else if pages == tt.matchPage {
						g.Expect(json.NewEncoder(w).Encode([]any{map[string]any{
							"id": 42, "author": map[string]string{"username": "flux"}, "body": key,
						}})).To(Succeed())
					} else {
						// Neither another user's marker nor an unrelated Flux note matches.
						g.Expect(json.NewEncoder(w).Encode([]any{
							map[string]any{"id": 1, "author": map[string]string{"username": "reviewer"}, "body": key},
							map[string]any{"id": 2, "author": map[string]string{"username": "flux"}, "body": "unrelated"},
						})).To(Succeed())
					}
					return
				}
				writes++
				g.Expect(r.Method).To(Equal(tt.wantMethod))
				if r.Method == http.MethodPut {
					g.Expect(r.URL.Path).To(Equal("/api/v4/projects/foo/bar/merge_requests/7/notes/42"))
				}
				var note struct{ Body string }
				g.Expect(json.NewDecoder(r.Body).Decode(&note)).To(Succeed())
				g.Expect(note.Body).To(ContainSubstring(key))
				g.Expect(note.Body).To(ContainSubstring(event.Message))
				if tt.writeError != "" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = fmt.Fprint(w, `{"message":"write failed"}`)
					return
				}
				_, _ = fmt.Fprint(w, `{}`)
			}))
			t.Cleanup(srv.Close)
			n, err := NewFactory(context.Background(), WithURL(srv.URL+"/foo/bar"),
				WithToken("token"), WithProviderUID("provider"), WithChangeRequestComment(tt.config)).Notifier(apiv1.GitLabMergeRequestCommentProvider)
			g.Expect(err).NotTo(HaveOccurred())
			err = n.Post(ctx, event)
			if tt.cancelPage != 0 {
				<-cancelled
				g.Expect(err).To(MatchError(ContainSubstring("failed to list merge request notes")))
				g.Expect(err).To(MatchError(ContainSubstring(context.Canceled.Error())))
				g.Expect(writes).To(BeZero())
			} else if tt.failPage != 0 {
				g.Expect(err).To(MatchError(ContainSubstring("failed to list merge request notes")))
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
