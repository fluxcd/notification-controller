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

	"github.com/google/go-github/v41/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitHubDispatchBasic(t *testing.T) {
	g, err := NewGitHubDispatch("https://github.com/foo/bar", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "api.github.com")
}

func TestNewEnterpriseGitHubDispatchBasic(t *testing.T) {
	g, err := NewGitHubDispatch("https://foobar.com/foo/bar", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "foobar.com")
}

func TestNewGitHubDispatchInvalidUrl(t *testing.T) {
	_, err := NewGitHubDispatch("https://github.com/foo/bar/baz", "foobar", nil)
	assert.NotNil(t, err)
}

func TestNewGitHubDispatchEmptyToken(t *testing.T) {
	_, err := NewGitHubDispatch("https://github.com/foo/bar", "", nil)
	assert.NotNil(t, err)
}

func TestGitHubDispatch_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload = github.DispatchRequestOptions{}
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)
		require.Equal(t, "webapp/gitops-system", payload.EventType)
	}))
	defer ts.Close()

	githubDispatch, err := NewGitHubDispatch(ts.URL, "foobar", nil)
	require.NoError(t, err)

	err = githubDispatch.Post(testEvent())
	require.NoError(t, err)
}

func TestGitHubDispatch_PostUpdate(t *testing.T) {
	githubDispatch, err := NewGitHubDispatch("https://github.com/foo/bar", "foobar", nil)
	require.NoError(t, err)

	event := testEvent()
	event.Metadata["commit_status"] = "update"
	err = githubDispatch.Post(event)
	require.NoError(t, err)
}
