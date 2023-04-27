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

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestUtil_NameAndDescription(t *testing.T) {
	event := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
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
	tests := []struct {
		name     string
		revision string
		expect   string
		wantErr  string
	}{
		{
			name:     "commit",
			revision: "sha1:a1afe267b54f38b46b487f6e938a6fd508278c07",
			expect:   "a1afe267b54f38b46b487f6e938a6fd508278c07",
		},
		{
			name:     "branch",
			revision: "master@sha1:a1afe267b54f38b46b487f6e938a6fd508278c07",
			expect:   "a1afe267b54f38b46b487f6e938a6fd508278c07",
		},
		{
			name:     "nested branch",
			revision: "environment/dev@sha1:a1afe267b54f38b46b487f6e938a6fd508278c07",
			expect:   "a1afe267b54f38b46b487f6e938a6fd508278c07",
		},
		{
			name:     "legacy",
			revision: "master/a1afe267b54f38b46b487f6e938a6fd508278c07",
			expect:   "a1afe267b54f38b46b487f6e938a6fd508278c07",
		},
		{
			name:     "legacy (nested branch)",
			revision: "environment/dev/a1afe267b54f38b46b487f6e938a6fd508278c07",
			expect:   "a1afe267b54f38b46b487f6e938a6fd508278c07",
		},
		{
			name:     "legacy (one component)",
			revision: "master",
			wantErr:  "failed to extract commit hash from 'master' revision",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rev, err := parseRevision(tt.revision)
			if tt.wantErr != "" {
				require.Error(t, err, tt.wantErr)
				require.Empty(t, rev)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expect, rev)
		})
	}
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

func TestUtil_ParseGitHttpWithSubgroup(t *testing.T) {
	addr := "https://gitlab.com/foo/bar/foo.git"
	host, id, err := parseGitAddress(addr)
	require.NoError(t, err)
	require.Equal(t, "https://gitlab.com", host)
	require.Equal(t, "foo/bar/foo", id)
}

func TestUtil_Sha1String(t *testing.T) {
	str := "kustomization/namespace-foo-and-service-bar"
	s := sha1String(str)
	require.Equal(t, "12ea142172e98435e16336acbbed8919610922c3", s)
}

func TestUtil_BasicAuth(t *testing.T) {
	username := "user"
	password := "password"
	s := basicAuth(username, password)
	require.Equal(t, "dXNlcjpwYXNzd29yZA==", s)
}

func TestUtil_GenerateCommitStatusID(t *testing.T) {
	statusIDTests := []struct {
		name        string
		providerUID string
		event       eventv1.Event
		want        string
	}{
		{
			name:        "simple event case",
			providerUID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
			event: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Reason: "ApplySucceeded",
			},
			want: "kustomization/gitops-system/0c9c2e41",
		},
	}

	for _, tt := range statusIDTests {
		t.Run(tt.name, func(t *testing.T) {
			id := generateCommitStatusID(tt.providerUID, tt.event)

			require.Equal(t, tt.want, id)
		})
	}
}
