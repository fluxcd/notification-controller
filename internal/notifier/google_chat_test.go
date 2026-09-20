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
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	. "github.com/onsi/gomega"
)

func TestNewGoogleChat(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		g := NewWithT(t)
		gc, err := NewGoogleChat("https://chat.googleapis.com/v1/spaces/test/messages?key=key&token=token", "")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gc.URL).To(Equal("https://chat.googleapis.com/v1/spaces/test/messages?key=key&token=token"))
		g.Expect(gc.ProxyURL).To(BeEmpty())
	})

	t.Run("with proxy", func(t *testing.T) {
		g := NewWithT(t)
		gc, err := NewGoogleChat("https://chat.googleapis.com/v1/spaces/test/messages", "http://proxy:8080")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gc.ProxyURL).To(Equal("http://proxy:8080"))
	})

	t.Run("invalid URL", func(t *testing.T) {
		g := NewWithT(t)
		_, err := NewGoogleChat("not a url", "")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid Google Chat hook URL"))
	})
}

func TestGoogleChat_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload = GoogleChatPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.Cards).To(HaveLen(1))
		g.Expect(payload.Cards[0].Header.Title).To(Equal("gitrepository/webapp.gitops-system"))
		g.Expect(payload.Cards[0].Header.SubTitle).To(Equal("source-controller"))
		g.Expect(payload.Cards[0].Sections[0].Widgets[0].TextParagraph.Text).To(Equal("message"))
		g.Expect(payload.Cards[0].Sections[1].Widgets[0].KeyValue.TopLabel).To(Equal("test"))
		g.Expect(payload.Cards[0].Sections[1].Widgets[0].KeyValue.Content).To(Equal("metadata"))
	}))
	defer ts.Close()

	google_chat, err := NewGoogleChat(ts.URL, "")
	g.Expect(err).ToNot(HaveOccurred())

	err = google_chat.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestGoogleChat_PostErrorSeverity(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload = GoogleChatPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.Cards[0].Sections[0].Widgets[0].TextParagraph.Text).To(ContainSubstring(`color="#ff0000"`))
		g.Expect(payload.Cards[0].Sections[0].Widgets[0].TextParagraph.Text).To(ContainSubstring("message"))
	}))
	defer ts.Close()

	gc, err := NewGoogleChat(ts.URL, "")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Severity = eventv1.EventSeverityError
	err = gc.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestGoogleChat_PostNoMetadata(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload = GoogleChatPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		// Only message section, no metadata section
		g.Expect(payload.Cards[0].Sections).To(HaveLen(1))
	}))
	defer ts.Close()

	gc, err := NewGoogleChat(ts.URL, "")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Metadata = nil
	err = gc.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestGoogleChat_PostServerError(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	gc, err := NewGoogleChat(ts.URL, "")
	g.Expect(err).ToNot(HaveOccurred())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = gc.Post(ctx, testEvent())
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("postMessage failed"))
}
