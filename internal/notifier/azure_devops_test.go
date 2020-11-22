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

func TestNewAzureDevOpsBasic(t *testing.T) {
	a, err := NewAzureDevOps("https://dev.azure.com/foo/bar/_git/baz.git", "foo")
	assert.Nil(t, err)
	assert.Equal(t, a.Project, "bar")
	assert.Equal(t, a.Repo, "baz")
}

func TestNewAzureDevOpsInvalidUrl(t *testing.T) {
	_, err := NewAzureDevOps("https://dev.azure.com/foo/bar/baz.git", "foo")
	assert.NotNil(t, err)
}

func TestNewAzureDevOpsMissingToken(t *testing.T) {
	_, err := NewAzureDevOps("https://dev.azure.com/foo/bar/baz.git", "")
	assert.NotNil(t, err)
}
