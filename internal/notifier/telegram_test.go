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

	"github.com/stretchr/testify/require"
)

func TestTelegram_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/sendMessage", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload TelegramPayload
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		require.Equal(t, "channel", payload.ChatID)
		require.Equal(t, "MarkdownV2", payload.ParseMode)

		lines := strings.Split(payload.Text, "\n")
		require.Len(t, lines, 5)
		slices.Sort(lines[2:4])
		require.Equal(t, "*💫 gitrepository/webapp/gitops\\-system*", lines[0])
		require.Equal(t, "message", lines[1])
		require.Equal(t, []string{
			"\\- *kubernetes\\.io/somekey*: some\\.value",
			"\\- *test*: metadata",
			"",
		}, lines[2:])

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	telegram, err := NewTelegram("", "", "channel", "token")
	require.NoError(t, err)

	telegram.URL = ts.URL

	ev := testEvent()
	ev.Metadata["kubernetes.io/somekey"] = "some.value"
	err = telegram.Post(context.TODO(), ev)
	require.NoError(t, err)
}

func TestTelegram_NewTelegram_IgnoresAddress(t *testing.T) {
	telegram, err := NewTelegram("https://api.telegram.org", "", "channel", "token")
	require.NoError(t, err)
	require.Equal(t, "https://api.telegram.org/bottoken", telegram.URL)

	telegram2, err := NewTelegram("https://custom.example.com", "", "channel", "token")
	require.NoError(t, err)
	require.Equal(t, "https://api.telegram.org/bottoken", telegram2.URL)

	telegram3, err := NewTelegram("", "", "channel", "token")
	require.NoError(t, err)
	require.Equal(t, "https://api.telegram.org/bottoken", telegram3.URL)
}
