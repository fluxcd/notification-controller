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

func TestNewLark(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		g := NewWithT(t)
		lark, err := NewLark("https://open.larksuite.com/open-apis/bot/v2/hook/test")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(lark.URL).To(Equal("https://open.larksuite.com/open-apis/bot/v2/hook/test"))
	})

	t.Run("invalid URL", func(t *testing.T) {
		g := NewWithT(t)
		_, err := NewLark("not a url")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid"))
	})

	t.Run("empty URL", func(t *testing.T) {
		g := NewWithT(t)
		_, err := NewLark("")
		g.Expect(err).To(HaveOccurred())
	})
}

func TestLark_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload LarkPayload
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.MsgType).To(Equal("interactive"))
		g.Expect(payload.Card.Config.WideScreenMode).To(BeTrue())
		g.Expect(payload.Card.Header.Title.Tag).To(Equal("plain_text"))
		g.Expect(payload.Card.Header.Title.Content).To(Equal("💫 gitrepository/webapp.gitops-system"))
		g.Expect(payload.Card.Header.Template).To(Equal("turquoise"))
		g.Expect(payload.Card.Elements).To(HaveLen(1))
		g.Expect(payload.Card.Elements[0].Tag).To(Equal("div"))
		g.Expect(payload.Card.Elements[0].Text.Tag).To(Equal("lark_md"))
		g.Expect(payload.Card.Elements[0].Text.Content).To(ContainSubstring("message"))
		g.Expect(payload.Card.Elements[0].Text.Content).To(ContainSubstring("test: metadata"))
	}))
	defer ts.Close()

	lark, err := NewLark(ts.URL)
	g.Expect(err).ToNot(HaveOccurred())

	err = lark.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestLark_PostErrorSeverity(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload LarkPayload
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.Card.Header.Title.Content).To(HavePrefix("🚨"))
		g.Expect(payload.Card.Header.Template).To(Equal("red"))
	}))
	defer ts.Close()

	lark, err := NewLark(ts.URL)
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Severity = eventv1.EventSeverityError
	err = lark.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestLark_PostServerError(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	lark, err := NewLark(ts.URL)
	g.Expect(err).ToNot(HaveOccurred())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = lark.Post(ctx, testEvent())
	g.Expect(err).To(HaveOccurred())
}
