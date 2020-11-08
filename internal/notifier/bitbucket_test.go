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

func TestNewBitbucketBasic(t *testing.T) {
	b, err := NewBitbucket("https://bitbucket.org/foo/bar", "foo:bar")

	assert.Nil(t, err)
	assert.Equal(t, b.Owner, "foo")
	assert.Equal(t, b.Repo, "bar")
}

func TestNewBitbucketInvalidUrl(t *testing.T) {
	_, err := NewBitbucket("https://bitbucket.org/foo/bar/baz", "foo:bar")
	assert.NotNil(t, err)
}

func TestNewBitbucketInvalidToken(t *testing.T) {
	_, err := NewBitbucket("https://bitbucket.org/foo/bar", "bar")
	assert.NotNil(t, err)
}
