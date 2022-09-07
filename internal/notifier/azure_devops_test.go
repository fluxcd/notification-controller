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
	"strings"
	"testing"

	"github.com/fluxcd/pkg/runtime/events"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
	"github.com/stretchr/testify/assert"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func TestNewAzureDevOpsBasic(t *testing.T) {
	a, err := NewAzureDevOps("https://dev.azure.com/foo/bar/_git/baz", "foo", nil)
	assert.Nil(t, err)
	assert.Equal(t, a.Project, "bar")
	assert.Equal(t, a.Repo, "baz")
}

func TestNewAzureDevOpsInvalidUrl(t *testing.T) {
	_, err := NewAzureDevOps("https://dev.azure.com/foo/bar/baz", "foo", nil)
	assert.NotNil(t, err)
}

func TestNewAzureDevOpsMissingToken(t *testing.T) {
	_, err := NewAzureDevOps("https://dev.azure.com/foo/bar/baz", "", nil)
	assert.NotNil(t, err)
}

func TestDuplicateAzureDevOpsStatus(t *testing.T) {
	assert := assert.New(t)

	var tests = []struct {
		ss  *[]git.GitStatus
		s   *git.GitStatus
		dup bool
	}{
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar"), true},
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Failed, "foo", "bar"), false},
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Succeeded, "baz", "bar"), false},
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Succeeded, "foo", "baz"), false},
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "baz", "bar"), *azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar"), true},
	}

	for _, test := range tests {
		assert.Equal(test.dup, duplicateAzureDevOpsStatus(test.ss, test.s))
	}
}

const apiLocations = `{"count":0,"value":[{"area":"","id":"428dd4fb-fda5-4722-af02-9313b80305da","routeTemplate":"","resourceName":"","maxVersion":"6.0","minVersion":"5.0","releasedVersion":"6.0"}]}`

func Fuzz_AzureDevOps(f *testing.F) {
	f.Add("alakazam", "org/proj/_git/repo", "revision/dsa123a", "error", "", []byte{}, []byte(`{"count":1,"value":[{"state":"error","description":"","context":{"genre":"fluxcd","name":"/"}}]}`))
	f.Add("alakazam", "org/proj/_git/repo", "revision/dsa123a", "info", "", []byte{}, []byte(`{"count":1,"value":[{"state":"info","description":"","context":{"genre":"fluxcd","name":"/"}}]}`))
	f.Add("alakazam", "org/proj/_git/repo", "revision/dsa123a", "info", "", []byte{}, []byte(`{"count":0,"value":[]}`))
	f.Add("alakazam", "org/proj/_git/repo", "", "", "Progressing", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		token, urlSuffix, revision, severity, reason string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "_apis") {
				w.Write([]byte(apiLocations))
			} else {
				w.Write(response)
			}

			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		azureDevOps, err := NewAzureDevOps(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), token, &cert)
		if err != nil {
			return
		}

		event := events.Event{}

		// Try to fuzz the event object, but if it fails (not enough seed),
		// ignore it, as other inputs are also being used in this test.
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil && (revision != "") {
			event.Metadata = map[string]string{
				"revision": revision,
			}
		}
		event.Severity = severity

		_ = azureDevOps.Post(context.TODO(), event)
	})
}

func azStatus(state git.GitStatusState, context string, description string) *git.GitStatus {
	genre := "fluxcd"
	return &git.GitStatus{
		Context: &git.GitStatusContext{
			Name:  &context,
			Genre: &genre,
		},
		Description: &description,
		State:       &state,
	}
}
