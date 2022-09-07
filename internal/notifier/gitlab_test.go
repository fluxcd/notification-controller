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
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/stretchr/testify/assert"
)

func TestNewGitLabBasic(t *testing.T) {
	g, err := NewGitLab("https://gitlab.com/foo/bar", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Id, "foo/bar")
}

func TestNewGitLabSubgroups(t *testing.T) {
	g, err := NewGitLab("https://gitlab.com/foo/bar/baz", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Id, "foo/bar/baz")
}

func TestNewGitLabSelfHosted(t *testing.T) {
	g, err := NewGitLab("https://example.com/foo/bar", "foo:bar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Id, "foo/bar")
	assert.Equal(t, g.Client.BaseURL().Host, "example.com")
}

func TestNewGitLabEmptyToken(t *testing.T) {
	_, err := NewGitLab("https://gitlab.com/foo/bar", "", nil)
	assert.NotNil(t, err)
}

func Fuzz_GitLab(f *testing.F) {
	f.Add("token", "org/repo", "revision/abce1", "error", "", []byte{}, []byte(`[{"sha":"abce1","status":"failed","name":"/","description":""}]`))
	f.Add("token", "org/repo", "revision/abce1", "info", "", []byte{}, []byte(`[{"sha":"abce1","status":"failed","name":"/","description":""}]`))
	f.Add("token", "org/repo", "revision/abce1", "info", "Progressing", []byte{}, []byte{})
	f.Add("token", "org/repo", "revision/abce1", "info", "", []byte{}, []byte(`[]`))

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

		gitLab, err := NewGitLab(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), token, &cert)
		if err != nil {
			return
		}

		event := events.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil && (revision != "") {
			event.Metadata = map[string]string{
				"revision": revision,
			}
		}

		event.Severity = severity
		event.Reason = reason

		_ = gitLab.Post(context.TODO(), event)
	})
}
