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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
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

	gl, err := NewGitLabMergeRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", srv.URL+"/foo/bar", "foobar", nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gl.ProjectID).To(Equal("foo/bar"))
	g.Expect(gl.ProviderUID).To(Equal("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a"))
	g.Expect(gl.Username).To(Equal("test-user"))
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

	gl, err := NewGitLabMergeRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", srv.URL+"/foo/bar/baz", "foobar", nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gl.ProjectID).To(Equal("foo/bar/baz"))
}

func TestNewGitLabMergeRequestCommentEmptyToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitLabMergeRequestComment("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "https://gitlab.com/foo/bar", "", nil)
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitLabMergeRequestCommentEmptyProviderUID(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitLabMergeRequestComment("", "https://gitlab.com/foo/bar", "foobar", nil)
	g.Expect(err).To(HaveOccurred())
}
