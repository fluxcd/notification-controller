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
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/google/go-github/v41/github"
	"github.com/stretchr/testify/assert"
)

func TestNewGitHubBasic(t *testing.T) {
	g, err := NewGitHub("https://github.com/foo/bar", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "api.github.com")
}

func TestNewEmterpriseGitHubBasic(t *testing.T) {
	g, err := NewGitHub("https://foobar.com/foo/bar", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.Client.BaseURL.Host, "foobar.com")
}

func TestNewGitHubInvalidUrl(t *testing.T) {
	_, err := NewGitHub("https://github.com/foo/bar/baz", "foobar", nil)
	assert.NotNil(t, err)
}

func TestNewGitHubEmptyToken(t *testing.T) {
	_, err := NewGitHub("https://github.com/foo/bar", "", nil)
	assert.NotNil(t, err)
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

func Fuzz_GitHub(f *testing.F) {
	f.Add("token", "org/repo", "revision/abce1", "error", "", []byte{}, []byte(`[{"context":"/","state":"failure","description":""}]`))
	f.Add("token", "org/repo", "revision/abce1", "info", "", []byte{}, []byte(`[{"context":"/","state":"success","description":""}]`))
	f.Add("token", "org/repo", "revision/abce1", "info", "", []byte{}, []byte(`[{"context":"/","state":"failure","description":""}]`))
	f.Add("token", "org/repo", "revision/abce1", "info", "", []byte{}, []byte(`[{"context":"/"}]`))
	f.Add("token", "org/repo", "revision/abce1", "info", "", []byte{}, []byte(`[{}]`))
	f.Add("token", "org/repo", "revision/abce1", "info", "Progressing", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		token, urlSuffix, revision, severity, reason string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		github, err := NewGitHub(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), token, &cert)
		if err != nil {
			return
		}

		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil && (revision != "") {
			event.Metadata = map[string]string{
				"revision": revision,
			}
		}

		event.Severity = severity
		event.Reason = reason

		_ = github.Post(context.TODO(), event)
	})
}

func ghStatus(state string, context string, description string) *github.RepoStatus {
	return &github.RepoStatus{
		State:       &state,
		Context:     &context,
		Description: &description,
	}
}
