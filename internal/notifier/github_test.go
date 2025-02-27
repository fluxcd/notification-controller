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
	"github.com/stretchr/testify/assert"
)

func TestNewGitHubBasic(t *testing.T) {
	g, err := NewGitHub("kustomization/gitops-system/0c9c2e41", "https://github.com/foo/bar", "foobar", nil, "", "", "", nil, nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "api.github.com")
	assert.Equal(t, g.CommitStatus, "kustomization/gitops-system/0c9c2e41")
}

func TestNewEmterpriseGitHubBasic(t *testing.T) {
	g, err := NewGitHub("kustomization/gitops-system/0c9c2e41", "https://foobar.com/foo/bar", "foobar", nil, "", "", "", nil, nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "foobar.com")
	assert.Equal(t, g.CommitStatus, "kustomization/gitops-system/0c9c2e41")
}

func TestNewGitHubInvalidUrl(t *testing.T) {
	_, err := NewGitHub("kustomization/gitops-system/0c9c2e41", "https://github.com/foo/bar/baz", "foobar", nil, "", "", "", nil, nil)
	assert.NotNil(t, err)
}

func TestNewGitHubEmptyToken(t *testing.T) {
	_, err := NewGitHub("kustomization/gitops-system/0c9c2e41", "https://github.com/foo/bar", "", nil, "", "", "", nil, nil)
	assert.NotNil(t, err)
}

func TestNewGitHubEmptyCommitStatus(t *testing.T) {
	_, err := NewGitHub("", "https://github.com/foo/bar", "foobar", nil, "", "", "", nil, nil)
	assert.NotNil(t, err)
}

func TestNewGithubProvider(t *testing.T) {
	appID := "123"
	installationID := "456"
	kp, _ := ssh.GenerateKeyPair(ssh.RSA_4096)
	expiresAt := time.Now().UTC().Add(time.Hour)

	var tests = []struct {
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
			wantErr: errors.New("app ID must be provided to use github app authentication"),
		},
		{
			name: "provider with missing app installation ID in options ",
			secretData: map[string][]byte{
				"githubAppID":         []byte(appID),
				"githubAppPrivateKey": kp.PrivateKey,
			},
			wantErr: errors.New("app installation ID must be provided to use github app authentication"),
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
	}

	for _, tt := range tests {
		handler := func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			var response []byte
			var err error
			response, err = json.Marshal(&authgithub.AppToken{Token: "access-token", ExpiresAt: expiresAt})
			assert.Nil(t, err)
			w.Write(response)
		}
		srv := httptest.NewServer(http.HandlerFunc(handler))
		t.Cleanup(func() {
			srv.Close()
		})

		if tt.secretData != nil && len(tt.secretData) > 0 {
			tt.secretData["githubAppBaseURL"] = []byte(srv.URL)
		}
		_, err := NewGitHub("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "https://github.com/foo/bar", "", nil, "", "foo", "bar", tt.secretData, nil)
		if tt.wantErr != nil {
			assert.NotNil(t, err)
			assert.Equal(t, tt.wantErr, err)
		} else {
			assert.Nil(t, err)
		}
	}
}

func TestDuplicateGithubStatus(t *testing.T) {
	assert := assert.New(t)

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
		assert.Equal(test.dup, duplicateGithubStatus(test.ss, test.s))
	}
}

func ghStatus(state string, context string, description string) *github.RepoStatus {
	return &github.RepoStatus{
		State:       &state,
		Context:     &context,
		Description: &description,
	}
}
