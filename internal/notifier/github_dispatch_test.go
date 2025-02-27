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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	authgithub "github.com/fluxcd/pkg/auth/github"
	"github.com/fluxcd/pkg/ssh"
)

func TestNewGitHubDispatchBasic(t *testing.T) {
	g, err := NewGitHubDispatch("https://github.com/foo/bar", "foobar", nil, nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "api.github.com")
}

func TestNewEnterpriseGitHubDispatchBasic(t *testing.T) {
	g, err := NewGitHubDispatch("https://foobar.com/foo/bar", "foobar", nil, nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "foobar.com")
}

func TestNewGitHubDispatchInvalidUrl(t *testing.T) {
	_, err := NewGitHubDispatch("https://github.com/foo/bar/baz", "foobar", nil, nil)
	assert.NotNil(t, err)
}

func TestNewGitHubDispatchEmptyToken(t *testing.T) {
	_, err := NewGitHubDispatch("https://github.com/foo/bar", "", nil, nil)
	assert.NotNil(t, err)
}

func TestNewGithubDispatchProvider(t *testing.T) {
	appID := "123"
	installationID := "456"
	kp, _ := ssh.GenerateKeyPair(ssh.RSA_4096)
	expiresAt := time.Now().UTC().Add(time.Hour)

	var tests = []struct {
		name    string
		opts    *ProviderOptions
		wantErr error
	}{
		{
			name:    "nil provider, no token",
			opts:    nil,
			wantErr: errors.New("github token or github app details must be specified"),
		},
		{
			name:    "provider with no github options",
			opts:    &ProviderOptions{Name: ProviderGitHub},
			wantErr: errors.New("github token or github app details must be specified"),
		},
		{
			name:    "provider with empty github options",
			opts:    &ProviderOptions{Name: ProviderGitHub, GitHubOpts: []authgithub.OptFunc{}},
			wantErr: errors.New("github token or github app details must be specified"),
		},
		{
			name:    "provider with incorrect name",
			opts:    &ProviderOptions{Name: "azure", GitHubOpts: []authgithub.OptFunc{authgithub.WithAppID("123")}},
			wantErr: errors.New("invalid provider name azure"),
		},
		{
			name: "provider with missing app ID in options ",
			opts: &ProviderOptions{
				Name:       ProviderGitHub,
				GitHubOpts: []authgithub.OptFunc{authgithub.WithInstllationID(installationID), authgithub.WithPrivateKey(kp.PrivateKey)},
			},
			wantErr: errors.New("app ID must be provided to use github app authentication"),
		},
		{
			name: "provider with missing app installation ID in options ",
			opts: &ProviderOptions{
				Name:       ProviderGitHub,
				GitHubOpts: []authgithub.OptFunc{authgithub.WithAppID(appID), authgithub.WithPrivateKey(kp.PrivateKey)},
			},
			wantErr: errors.New("app installation ID must be provided to use github app authentication"),
		},
		{
			name: "provider with missing app private key in options ",
			opts: &ProviderOptions{
				Name:       ProviderGitHub,
				GitHubOpts: []authgithub.OptFunc{authgithub.WithAppID(appID), authgithub.WithInstllationID(installationID)},
			},
			wantErr: errors.New("private key must be provided to use github app authentication"),
		},
		{
			name: "provider with complete app authentication information",
			opts: &ProviderOptions{
				Name:       ProviderGitHub,
				GitHubOpts: []authgithub.OptFunc{authgithub.WithAppID(appID), authgithub.WithInstllationID(installationID), authgithub.WithPrivateKey(kp.PrivateKey)},
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

		if tt.opts != nil && tt.opts.GitHubOpts != nil && len(tt.opts.GitHubOpts) > 0 {
			tt.opts.GitHubOpts = append(tt.opts.GitHubOpts, authgithub.WithAppBaseURL(srv.URL))
		}
		_, err := NewGitHubDispatch("https://github.com/foo/bar", "", nil, tt.opts)
		if tt.wantErr != nil {
			assert.NotNil(t, err)
			assert.Equal(t, tt.wantErr, err)
		} else {
			assert.Nil(t, err)
		}
	}
}

func TestGitHubDispatch_PostUpdate(t *testing.T) {
	githubDispatch, err := NewGitHubDispatch("https://github.com/foo/bar", "foobar", nil, nil)
	require.NoError(t, err)

	event := testEvent()
	event.Metadata[eventv1.MetaCommitStatusKey] = eventv1.MetaCommitStatusUpdateValue
	err = githubDispatch.Post(context.TODO(), event)
	require.NoError(t, err)
}
