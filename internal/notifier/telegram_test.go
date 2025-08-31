/*
Copyright 2024 The Flux authors

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
	"slices"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestTelegram_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.Method).To(Equal(http.MethodPost))
		g.Expect(r.URL.Path).To(Equal("/sendMessage"))
		g.Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload TelegramPayload
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.ChatID).To(Equal("channel"))
		g.Expect(payload.ParseMode).To(Equal("MarkdownV2"))

		lines := strings.Split(payload.Text, "\n")
		g.Expect(lines).To(HaveLen(5))
		slices.Sort(lines[2:4])
		g.Expect(lines[0]).To(Equal("*💫 gitrepository/webapp/gitops\\-system*"))
		g.Expect(lines[1]).To(Equal("message"))
		g.Expect(lines[2:]).To(Equal([]string{
			"\\- *kubernetes\\.io/somekey*: some\\.value",
			"\\- *test*: metadata",
			"",
		}))

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	telegram, err := NewTelegram("", "channel", "token")
	g.Expect(err).ToNot(HaveOccurred())

	telegram.url = ts.URL

	ev := testEvent()
	ev.Metadata["kubernetes.io/somekey"] = "some.value"
	err = telegram.Post(context.TODO(), ev)
	g.Expect(err).ToNot(HaveOccurred())
}
