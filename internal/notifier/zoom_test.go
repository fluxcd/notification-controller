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
	"testing"

	. "github.com/onsi/gomega"
)

func TestZoom_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Query().Get("format")).To(Equal("full"))
		g.Expect(r.Header.Get("Authorization")).To(Equal("token"))
		g.Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload = ZoomPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.Content.Head.Text).To(Equal("gitrepository/webapp.gitops-system"))
		g.Expect(payload.Content.Head.SubHead.Text).To(Equal("info"))
		g.Expect(payload.Content.Body[0].Type).To(Equal("message"))
		g.Expect(payload.Content.Body[0].Text).To(Equal("message"))
		g.Expect(payload.Content.Body[1].Type).To(Equal("fields"))
		g.Expect(payload.Content.Body[1].Items[0].Key).To(Equal("test"))
		g.Expect(payload.Content.Body[1].Items[0].Value).To(Equal("metadata"))
	}))
	defer ts.Close()

	zoom, err := NewZoom(ts.URL, nil, nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	err = zoom.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestZoom_PostFormatPreserved(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Query().Get("format")).To(Equal("message"))
	}))
	defer ts.Close()

	zoom, err := NewZoom(ts.URL+"?format=message", nil, nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	err = zoom.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestNewZoom(t *testing.T) {
	g := NewWithT(t)

	_, err := NewZoom("invalid-url", nil, nil, "token")
	g.Expect(err).To(MatchError(ContainSubstring("invalid Zoom incoming webhook URL")))

	_, err = NewZoom("https://integrations.zoom.us/chat/webhooks/incomingwebhook/id", nil, nil, "")
	g.Expect(err).To(MatchError(ContainSubstring("empty Zoom verification token")))
}
