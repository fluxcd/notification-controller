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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v64/github"
	. "github.com/onsi/gomega"

	authgithub "github.com/fluxcd/pkg/git/github"
	"github.com/fluxcd/pkg/ssh"
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

	gh, err := NewGitHubPullRequestComment(context.Background(), "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		WithGitHubAddress(srv.URL+"/foo/bar"),
		WithGitHubToken("foobar"),
	)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gh.Owner).To(Equal("foo"))
	g.Expect(gh.Repo).To(Equal("bar"))
	g.Expect(gh.ProviderUID).To(Equal("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a"))
	g.Expect(gh.UserLogin).To(Equal("test-user"))
}

func TestNewGitHubPullRequestCommentInvalidUrl(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitHubPullRequestComment(context.Background(), "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		WithGitHubAddress("https://github.com/foo/bar/baz"),
		WithGitHubToken("foobar"),
	)
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitHubPullRequestCommentEmptyToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitHubPullRequestComment(context.Background(), "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		WithGitHubAddress("https://github.com/foo/bar"),
	)
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitHubPullRequestCommentEmptyProviderUID(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitHubPullRequestComment(context.Background(), "",
		WithGitHubAddress("https://github.com/foo/bar"),
		WithGitHubToken("foobar"),
	)
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
				response, err = json.Marshal(&authgithub.AppToken{
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
			_, err := NewGitHubPullRequestComment(context.Background(), "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				WithGitHubAddress("https://github.com/foo/bar"),
				WithGitHubProvider("foo", "bar"),
				WithGitHubSecretData(tt.secretData),
			)
			if tt.wantErr != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(Equal(tt.wantErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
