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
	"testing"

	. "github.com/onsi/gomega"
)

func TestNewGitLabBasic(t *testing.T) {
	gomega := NewWithT(t)
	n, err := NewGitLab(t.Context(), &notifierOptions{
		CommitStatus: "kustomization/gitops-system/0c9c2e41",
		URL:          "https://gitlab.com/foo/bar",
		Token:        "foobar",
	})
	gomega.Expect(err).ToNot(HaveOccurred())
	gitlab := n.(*GitLab)
	gomega.Expect(gitlab.Id).To(Equal("foo/bar"))
	gomega.Expect(gitlab.CommitStatus).To(Equal("kustomization/gitops-system/0c9c2e41"))
}

func TestNewGitLabSubgroups(t *testing.T) {
	gomega := NewWithT(t)
	n, err := NewGitLab(t.Context(), &notifierOptions{
		CommitStatus: "kustomization/gitops-system/0c9c2e41",
		URL:          "https://gitlab.com/foo/bar/baz",
		Token:        "foobar",
	})
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(n.(*GitLab).Id).To(Equal("foo/bar/baz"))
}

func TestNewGitLabSelfHosted(t *testing.T) {
	gomega := NewWithT(t)
	n, err := NewGitLab(t.Context(), &notifierOptions{
		CommitStatus: "kustomization/gitops-system/0c9c2e41",
		URL:          "https://example.com/foo/bar",
		Token:        "foo:bar",
	})
	gomega.Expect(err).ToNot(HaveOccurred())
	gitlab := n.(*GitLab)
	gomega.Expect(gitlab.Id).To(Equal("foo/bar"))
	gomega.Expect(gitlab.Client.BaseURL().Host).To(Equal("example.com"))
}

func TestNewGitLabEmptyToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitLab(t.Context(), &notifierOptions{CommitStatus: "kustomization/gitops-system/0c9c2e41", URL: "https://gitlab.com/foo/bar"})
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitLabEmptyCommitStatus(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitLab(t.Context(), &notifierOptions{URL: "https://gitlab.com/foo/bar", Token: "foobar"})
	g.Expect(err).To(HaveOccurred())
}
