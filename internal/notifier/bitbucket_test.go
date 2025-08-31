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

func TestNewBitbucketBasic(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucket("kustomization/gitops-system/0c9c2e41", "https://bitbucket.org/foo/bar", "foo:bar", nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(b.Owner).To(Equal("foo"))
	g.Expect(b.Repo).To(Equal("bar"))
	g.Expect(b.CommitStatus).To(Equal("kustomization/gitops-system/0c9c2e41"))
}

func TestNewBitbucketEmptyCommitStatus(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucket("", "https://bitbucket.org/foo/bar", "foo:bar", nil)
	g.Expect(err).To(HaveOccurred())
}

func TestNewBitbucketInvalidUrl(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucket("kustomization/gitops-system/0c9c2e41", "https://bitbucket.org/foo/bar/baz", "foo:bar", nil)
	g.Expect(err).To(HaveOccurred())
}

func TestNewBitbucketInvalidToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucket("kustomization/gitops-system/0c9c2e41", "https://bitbucket.org/foo/bar", "bar", nil)
	g.Expect(err).To(HaveOccurred())
}
