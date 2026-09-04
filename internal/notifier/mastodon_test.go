/*
Copyright 2026 The Flux authors

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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestMastodon_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Path).To(Equal("/api/v1/statuses"))
		g.Expect(r.Header.Get("Authorization")).To(Equal("Bearer token"))
		g.Expect(r.Header.Get("Idempotency-Key")).ToNot(BeEmpty())
		g.Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload = MastodonPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.Status).To(ContainSubstring("💫 gitrepository/webapp.gitops-system"))
		g.Expect(payload.Status).To(ContainSubstring("message"))
		g.Expect(payload.Status).To(ContainSubstring("test: metadata"))
		g.Expect(payload.Visibility).To(BeEmpty())
	}))
	defer ts.Close()

	mastodon, err := NewMastodon(ts.URL, "", nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	err = mastodon.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestMastodon_PostVisibilityAndErrorSeverity(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Path).To(Equal("/api/v1/statuses"))
		g.Expect(r.URL.Query().Get("visibility")).To(BeEmpty())

		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload = MastodonPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.Visibility).To(Equal("unlisted"))
		g.Expect(payload.Status).To(ContainSubstring("🚨"))
	}))
	defer ts.Close()

	mastodon, err := NewMastodon(ts.URL+"?visibility=unlisted", "", nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Severity = "error"
	err = mastodon.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestMastodon_PostStatusTruncated(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload = MastodonPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		runes := []rune(payload.Status)
		g.Expect(len(runes)).To(Equal(mastodonMaxChars))
		g.Expect(runes[len(runes)-1]).To(Equal('…'))
	}))
	defer ts.Close()

	mastodon, err := NewMastodon(ts.URL, "", nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Message = strings.Repeat("z", 2*mastodonMaxChars)
	err = mastodon.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestNewMastodon(t *testing.T) {
	g := NewWithT(t)

	_, err := NewMastodon("invalid-url", "", nil, "token")
	g.Expect(err).To(MatchError(ContainSubstring("invalid Mastodon server URL")))

	_, err = NewMastodon("https://mastodon.social", "", nil, "")
	g.Expect(err).To(MatchError(ContainSubstring("empty Mastodon access token")))

	// The statuses path is preserved when already present in the address.
	m, err := NewMastodon("https://mastodon.social/api/v1/statuses", "", nil, "token")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(m.URL).To(Equal("https://mastodon.social/api/v1/statuses"))
}

// Go randomises map iteration, so the metadata lines shuffled between statuses
// and the same four keys arrived in a different order every time. Ten posts of
// one event is enough for an unsorted map to disagree with itself.
func TestMastodon_PostMetadataIsSorted(t *testing.T) {
	g := NewWithT(t)

	var got [][]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload = MastodonPayload{}
		g.Expect(json.Unmarshal(b, &payload)).To(Succeed())

		var keys []string
		for _, line := range strings.Split(payload.Status, "\n") {
			if k, _, ok := strings.Cut(line, ": "); ok {
				keys = append(keys, k)
			}
		}
		got = append(got, keys)
	}))
	defer ts.Close()

	mastodon, err := NewMastodon(ts.URL, "", nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Metadata = map[string]string{
		"revision":  "main/1234",
		"cluster":   "staging",
		"image-tag": "v1.2.3",
		"env":       "staging",
	}

	for i := 0; i < 10; i++ {
		g.Expect(mastodon.Post(context.TODO(), event)).To(Succeed())
	}

	want := []string{"cluster", "env", "image-tag", "revision"}
	for i, keys := range got {
		g.Expect(keys).To(Equal(want), "post %d returned metadata out of order", i)
	}
}
