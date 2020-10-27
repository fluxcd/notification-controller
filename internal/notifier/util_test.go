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

	"github.com/fluxcd/pkg/recorder"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestUtil_NameAndDescription(t *testing.T) {
	event := recorder.Event{
		InvolvedObject: v1.ObjectReference{
			Kind: "Kustomization",
			Name: "gitops-system",
		},
		Reason: "ApplySucceeded",
	}
	name, desc := formatNameAndDescription(event)
	require.Equal(t, "kustomization/gitops-system", name)
	require.Equal(t, "apply succeeded", desc)
}

func TestUtil_ParseRevision(t *testing.T) {
	revString := "master/a1afe267b54f38b46b487f6e938a6fd508278c07"
	rev, err := parseRevision(revString)
	require.NoError(t, err)
	require.Equal(t, "a1afe267b54f38b46b487f6e938a6fd508278c07", rev)
}

func TestUtil_ParseRevisionTooFewComponents(t *testing.T) {
	revString := "master/"
	_, err := parseRevision(revString)
	require.Error(t, err)
}

func TestUtil_ParseRevisionTooManyComponents(t *testing.T) {
	revString := "master/a1afe267b54f38b46b487f6e938a6fd508278c07/foo/bar"
	_, err := parseRevision(revString)
	require.Error(t, err)
}

func TestUtil_ParseGitHttps(t *testing.T) {
	addr := "https://github.com/foo/bar"
	host, id, err := parseGitAddress(addr)
	require.NoError(t, err)
	require.Equal(t, "https://github.com", host)
	require.Equal(t, "foo/bar", id)
}

func TestUtil_ParseGitCustomHost(t *testing.T) {
	addr := "https://example.com/foo/bar"
	host, id, err := parseGitAddress(addr)
	require.NoError(t, err)
	require.Equal(t, "https://example.com", host)
	require.Equal(t, "foo/bar", id)
}

func TestUtil_ParseGitHttpFileEnding(t *testing.T) {
	addr := "https://gitlab.com/foo/bar.git"
	host, id, err := parseGitAddress(addr)
	require.NoError(t, err)
	require.Equal(t, "https://gitlab.com", host)
	require.Equal(t, "foo/bar", id)
}

func TestUtil_ParseGitSsh(t *testing.T) {
	addr := "git@gitlab.com:foo/bar.git"
	host, id, err := parseGitAddress(addr)
	require.NoError(t, err)
	require.Equal(t, "https://gitlab.com", host)
	require.Equal(t, "foo/bar", id)
}

func TestUtil_ParseGitSshWithProtocol(t *testing.T) {
	addr := "ssh://git@github.com/stefanprodan/podinfo"
	host, id, err := parseGitAddress(addr)
	require.NoError(t, err)
	require.Equal(t, "https://github.com", host)
	require.Equal(t, "stefanprodan/podinfo", id)
}
