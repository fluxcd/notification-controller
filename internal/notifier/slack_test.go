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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestSlack_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload = SlackPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(payload.Attachments[0].AuthorName).To(Equal("gitrepository/webapp.gitops-system"))
		g.Expect(payload.Attachments[0].Fields[0].Value).To(Equal("metadata"))
	}))
	defer ts.Close()

	slack, err := NewSlack(ts.URL, nil, "", nil, "", "test")
	g.Expect(err).ToNot(HaveOccurred())

	err = slack.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestSlack_ValidateResponse(t *testing.T) {
	g := NewWithT(t)

	resp := httptest.NewRecorder()
	resp.Write([]byte(`{
  "ok": true
}`))
	err := validateSlackResponse(resp.Result())
	g.Expect(err).ToNot(HaveOccurred())

	resp = httptest.NewRecorder()
	resp.Write([]byte(`{
  "ok": false,
  "error": "too_many_attachments"
}`))
	err = validateSlackResponse(resp.Result())
	g.Expect(err).To(MatchError(ContainSubstring("Slack responded with error: too_many_attachments")))
}

func TestSlackLimitsResponseBody(t *testing.T) {
	g := NewWithT(t)
	server, written := newLargeResponseServer(http.StatusOK, `{"ok":false,"error":"`, `"}`)
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := postMessage(ctx, server.URL, nil, withResponseValidator(validateSlackResponse))
	g.Expect(err).To(MatchError(ContainSubstring("response body exceeds 1 MiB limit")))
	g.Eventually(written).Should(Receive(BeNumerically("<", 100*1024*1024)))
}

func TestSlackResponseBodyLimitBoundary(t *testing.T) {
	for _, size := range []int{1024 * 1024, 1024*1024 + 1} {
		g := NewWithT(t)
		const prefix, suffix = `{"ok":true,"error":"`, `"}`
		body := strings.NewReader(prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix)
		resp := &http.Response{Body: io.NopCloser(body)}
		err := validateSlackResponse(resp)
		g.Expect(resp.Body.Close()).To(Succeed())
		if size > 1024*1024 {
			g.Expect(err).To(MatchError("response body exceeds 1 MiB limit"))
		} else {
			g.Expect(err).ToNot(HaveOccurred())
		}
		g.Expect(body.Len()).To(BeZero())
	}
}

// Go randomises map iteration, so the fields shuffled between messages and the
// same four keys arrived in a different order every time. Ten posts of one event
// is enough for an unsorted map to disagree with itself.
func TestSlack_PostFieldsAreSorted(t *testing.T) {
	g := NewWithT(t)

	var got [][]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload SlackPayload
		g.Expect(json.Unmarshal(b, &payload)).To(Succeed())

		keys := make([]string, 0, len(payload.Attachments[0].Fields))
		for _, f := range payload.Attachments[0].Fields {
			keys = append(keys, f.Title)
		}
		got = append(got, keys)
	}))
	defer ts.Close()

	slack, err := NewSlack(ts.URL, nil, "", nil, "", "test")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Metadata = map[string]string{
		"revision":  "main/1234",
		"cluster":   "staging",
		"image-tag": "v1.2.3",
		"env":       "staging",
	}

	for i := 0; i < 10; i++ {
		g.Expect(slack.Post(context.TODO(), event)).To(Succeed())
	}

	want := []string{"cluster", "env", "image-tag", "revision"}
	for i, keys := range got {
		g.Expect(keys).To(Equal(want), "post %d returned fields out of order", i)
	}
}
