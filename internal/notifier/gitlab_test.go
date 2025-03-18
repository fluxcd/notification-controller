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

	"github.com/stretchr/testify/assert"
)

func TestNewGitLabBasic(t *testing.T) {
	g, err := NewGitLab("kustomization/gitops-system/0c9c2e41", "https://gitlab.com/foo/bar", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Id, "foo/bar")
	assert.Equal(t, g.CommitStatus, "kustomization/gitops-system/0c9c2e41")
}

func TestNewGitLabSubgroups(t *testing.T) {
	g, err := NewGitLab("kustomization/gitops-system/0c9c2e41", "https://gitlab.com/foo/bar/baz", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Id, "foo/bar/baz")
}

func TestNewGitLabSelfHosted(t *testing.T) {
	g, err := NewGitLab("kustomization/gitops-system/0c9c2e41", "https://example.com/foo/bar", "foo:bar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Id, "foo/bar")
	assert.Equal(t, g.Client.BaseURL().Host, "example.com")
}

func TestNewGitLabEmptyToken(t *testing.T) {
	_, err := NewGitLab("kustomization/gitops-system/0c9c2e41", "https://gitlab.com/foo/bar", "", nil)
	assert.NotNil(t, err)
}

func TestNewGitLabEmptyCommitStatus(t *testing.T) {
	_, err := NewGitLab("", "https://gitlab.com/foo/bar", "foobar", nil)
	assert.NotNil(t, err)
}
