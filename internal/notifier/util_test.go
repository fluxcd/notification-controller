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
	"unicode/utf8"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

func TestUtil_NameAndDescription(t *testing.T) {
	g := NewWithT(t)
	event := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind: "Kustomization",
			Name: "gitops-system",
		},
		Reason: "ApplySucceeded",
	}
	name, desc := formatNameAndDescription(event)
	g.Expect(name).To(Equal("kustomization/gitops-system"))
	g.Expect(desc).To(Equal("apply succeeded"))
}

const (
	longCommitStatusDescription      = "health check failed after 5m0s: timeout waiting for: [HelmRelease/apps/podinfo status: 'InProgress': Helm upgrade failed for release apps/podinfo with chart podinfo@6.7.0: context deadline exceeded]"
	shortenedCommitStatusDescription = "health check failed after 5m0s: timeout waiting for: [HelmRelease/app…ease apps/podinfo with chart podinfo@6.7.0: context deadline exceeded]"
)

func TestUtil_CommitStatusDescription(t *testing.T) {
	event := eventv1.Event{Reason: "ApplySucceeded"}
	for _, tt := range []struct {
		name   string
		custom string
		limit  int
		want   string
	}{
		{name: "empty", limit: 140, want: "apply succeeded"},
		{name: "whitespace only", custom: " \n\t ", limit: 140, want: "apply succeeded"},
		{name: "custom", custom: "Helm upgrade failed", limit: 140, want: "Helm upgrade failed"},
		{name: "multiple lines", custom: "  Helm upgrade failed:\n\n\tcontext  deadline exceeded \n", limit: 140, want: "Helm upgrade failed: context deadline exceeded"},
		{name: "too long", custom: longCommitStatusDescription, limit: 140, want: shortenedCommitStatusDescription},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(commitStatusDescription(event, tt.custom, tt.limit)).To(Equal(tt.want))
		})
	}
}

func TestUtil_TruncateMiddle(t *testing.T) {
	for _, tt := range []struct {
		name  string
		s     string
		limit int
		want  string
	}{
		{name: "shorter than limit", s: "abcdef", limit: 10, want: "abcdef"},
		{name: "exact limit", s: "abcdef", limit: 6, want: "abcdef"},
		{name: "limit exceeded by one", s: "abcdefg", limit: 6, want: "ab…efg"},
		{name: "odd limit", s: "abcdefghij", limit: 5, want: "ab…ij"},
		{name: "limit of two", s: "abc", limit: 2, want: "…c"},
		{name: "limit of one", s: "abc", limit: 1, want: "…"},
		{name: "limit of zero", s: "abc", limit: 0, want: ""},
		{name: "multi-byte runes within limit", s: "äöü€", limit: 4, want: "äöü€"},
		{name: "multi-byte runes", s: "äöüß€𝄞日本", limit: 5, want: "äö…日本"},
		{name: "example message", s: longCommitStatusDescription, limit: 140, want: shortenedCommitStatusDescription},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := truncateMiddle(tt.s, tt.limit)
			g.Expect(got).To(Equal(tt.want))
			g.Expect(utf8.ValidString(got)).To(BeTrue())
			g.Expect(utf8.RuneCountInString(got)).To(BeNumerically("<=", max(tt.limit, 0)))
		})
	}
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
			g := NewWithT(t)
			rev, err := parseRevision(tt.revision)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(rev).To(BeEmpty())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(rev).To(Equal(tt.expect))
		})
	}
}

func TestUtil_ParseGitHttps(t *testing.T) {
	g := NewWithT(t)
	addr := "https://github.com/foo/bar"
	host, id, err := parseGitAddress(addr)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(host).To(Equal("https://github.com"))
	g.Expect(id).To(Equal("foo/bar"))
}

func TestUtil_ParseGitCustomHost(t *testing.T) {
	g := NewWithT(t)
	addr := "https://example.com/foo/bar"
	host, id, err := parseGitAddress(addr)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(host).To(Equal("https://example.com"))
	g.Expect(id).To(Equal("foo/bar"))
}

func TestUtil_ParseGitHttpFileEnding(t *testing.T) {
	g := NewWithT(t)
	addr := "https://gitlab.com/foo/bar.git"
	host, id, err := parseGitAddress(addr)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(host).To(Equal("https://gitlab.com"))
	g.Expect(id).To(Equal("foo/bar"))
}

func TestUtil_ParseGitSsh(t *testing.T) {
	g := NewWithT(t)
	addr := "git@gitlab.com:foo/bar.git"
	host, id, err := parseGitAddress(addr)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(host).To(Equal("https://gitlab.com"))
	g.Expect(id).To(Equal("foo/bar"))
}

func TestUtil_ParseGitSshWithProtocol(t *testing.T) {
	g := NewWithT(t)
	addr := "ssh://git@github.com/stefanprodan/podinfo"
	host, id, err := parseGitAddress(addr)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(host).To(Equal("https://github.com"))
	g.Expect(id).To(Equal("stefanprodan/podinfo"))
}

func TestUtil_ParseGitHttpWithSubgroup(t *testing.T) {
	g := NewWithT(t)
	addr := "https://gitlab.com/foo/bar/foo.git"
	host, id, err := parseGitAddress(addr)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(host).To(Equal("https://gitlab.com"))
	g.Expect(id).To(Equal("foo/bar/foo"))
}

func TestUtil_Sha1String(t *testing.T) {
	g := NewWithT(t)
	str := "kustomization/namespace-foo-and-service-bar"
	s := sha1String(str)
	g.Expect(s).To(Equal("12ea142172e98435e16336acbbed8919610922c3"))
}

func TestUtil_BasicAuth(t *testing.T) {
	g := NewWithT(t)
	username := "user"
	password := "password"
	s := basicAuth(username, password)
	g.Expect(s).To(Equal("dXNlcjpwYXNzd29yZA=="))
}
