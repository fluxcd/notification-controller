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
	corev1 "k8s.io/api/core/v1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
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
