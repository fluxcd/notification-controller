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

func TestNewGitHubBasic(t *testing.T) {
	g, err := NewGitHub("https://github.com/foo/bar", "foobar")
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
}

func TestNewGitHubInvalidUrl(t *testing.T) {
	_, err := NewGitHub("https://github.com/foo/bar/baz", "foobar")
	assert.NotNil(t, err)
}

func TestNewGitHubEmptyToken(t *testing.T) {
	_, err := NewGitHub("https://github.com/foo/bar", "")
	assert.NotNil(t, err)
}
