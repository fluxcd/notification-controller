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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authgithub "github.com/fluxcd/pkg/git/github"
	"github.com/fluxcd/pkg/ssh"

	"github.com/google/go-github/v64/github"
	. "github.com/onsi/gomega"
)

func TestNewGitHubBasic(t *testing.T) {
	gm := NewWithT(t)
	g, err := NewGitHub("kustomization/gitops-system/0c9c2e41", "https://github.com/foo/bar", "foobar", nil, "", "", "", nil, nil)
	gm.Expect(err).ToNot(HaveOccurred())
	gm.Expect(g.Owner).To(Equal("foo"))
	gm.Expect(g.Repo).To(Equal("bar"))
	gm.Expect(g.Client.BaseURL.Host).To(Equal("api.github.com"))
	gm.Expect(g.CommitStatus).To(Equal("kustomization/gitops-system/0c9c2e41"))
}

func TestNewEmterpriseGitHubBasic(t *testing.T) {
	gm := NewWithT(t)
	g, err := NewGitHub("kustomization/gitops-system/0c9c2e41", "https://foobar.com/foo/bar", "foobar", nil, "", "", "", nil, nil)
	gm.Expect(err).ToNot(HaveOccurred())
	gm.Expect(g.Owner).To(Equal("foo"))
	gm.Expect(g.Repo).To(Equal("bar"))
	gm.Expect(g.Client.BaseURL.Host).To(Equal("foobar.com"))
	gm.Expect(g.CommitStatus).To(Equal("kustomization/gitops-system/0c9c2e41"))
}

func TestNewGitHubInvalidUrl(t *testing.T) {
	gm := NewWithT(t)
	_, err := NewGitHub("kustomization/gitops-system/0c9c2e41", "https://github.com/foo/bar/baz", "foobar", nil, "", "", "", nil, nil)
	gm.Expect(err).To(HaveOccurred())
}

func TestNewGitHubEmptyToken(t *testing.T) {
	gm := NewWithT(t)
	_, err := NewGitHub("kustomization/gitops-system/0c9c2e41", "https://github.com/foo/bar", "", nil, "", "", "", nil, nil)
	gm.Expect(err).To(HaveOccurred())
}

func TestNewGitHubEmptyCommitStatus(t *testing.T) {
	gm := NewWithT(t)
	_, err := NewGitHub("", "https://github.com/foo/bar", "foobar", nil, "", "", "", nil, nil)
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
				response, err = json.Marshal(&authgithub.AppToken{Token: "access-token", ExpiresAt: expiresAt})
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
			_, err := NewGitHub("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "https://github.com/foo/bar", "", nil, "", "foo", "bar", tt.secretData, nil)
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

func ghStatus(state string, context string, description string) *github.RepoStatus {
	return &github.RepoStatus{
		State:       &state,
		Context:     &context,
		Description: &description,
	}
}
