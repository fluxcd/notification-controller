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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestRSAKey(t *testing.T) []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	return pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
}

func TestNewGitHubDispatchBasic(t *testing.T) {
	g, err := NewGitHubDispatch("https://github.com/foo/bar", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "api.github.com")
}

func TestNewGitHubAppDispatchBasic(t *testing.T) {
	g, err := NewGitHubAppDispatch("https://github.com/foo/bar", 1, 1, generateTestRSAKey(t), nil)
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

func TestNewEnterpriseGitHubAppDispatchBasic(t *testing.T) {
	g, err := NewGitHubAppDispatch("https://foobar.com/foo/bar", 1, 1, generateTestRSAKey(t), nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "foobar.com")
}

func TestNewGitHubDispatchInvalidUrl(t *testing.T) {
	_, err := NewGitHubDispatch("https://github.com/foo/bar/baz", "foobar", nil)
	assert.NotNil(t, err)
}

func TestNewGitHubAppDispatchInvalidUrl(t *testing.T) {
	_, err := NewGitHubAppDispatch("https://github.com/foo/bar/baz", 1, 1, generateTestRSAKey(t), nil)
	assert.NotNil(t, err)
}

func TestNewGitHubDispatchEmptyToken(t *testing.T) {
	_, err := NewGitHubDispatch("https://github.com/foo/bar", "", nil)
	assert.NotNil(t, err)
}

func TestNewGitHubAppDispatchEmptyKey(t *testing.T) {
	_, err := NewGitHubAppDispatch("https://github.com/foo/bar", 1, 1, []byte{}, nil)
	assert.NotNil(t, err)
}

func TestGitHubDispatch_PostUpdate(t *testing.T) {
	githubDispatch, err := NewGitHubDispatch("https://github.com/foo/bar", "foobar", nil)
	require.NoError(t, err)

	event := testEvent()
	event.Metadata["commit_status"] = "update"
	err = githubDispatch.Post(context.TODO(), event)
	require.NoError(t, err)
}

func TestGitHubAppDispatch_PostUpdate(t *testing.T) {
	githubDispatch, err := NewGitHubAppDispatch("https://github.com/foo/bar", 1, 1, generateTestRSAKey(t), nil)
	require.NoError(t, err)

	event := testEvent()
	event.Metadata["commit_status"] = "update"
	err = githubDispatch.Post(context.TODO(), event)
	require.NoError(t, err)
}
