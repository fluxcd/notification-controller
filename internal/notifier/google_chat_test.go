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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/stretchr/testify/require"
)

func TestGoogleChat_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload = GoogleChatPayload{}
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		require.Equal(t, "gitrepository/webapp.gitops-system", payload.Cards[0].Header.Title)
		require.Equal(t, "source-controller", payload.Cards[0].Header.SubTitle)
		require.Equal(t, "message", payload.Cards[0].Sections[0].Widgets[0].TextParagraph.Text)
		require.Equal(t, "test", payload.Cards[0].Sections[1].Widgets[0].KeyValue.TopLabel)
		require.Equal(t, "metadata", payload.Cards[0].Sections[1].Widgets[0].KeyValue.Content)
	}))
	defer ts.Close()

	google_chat, err := NewGoogleChat(ts.URL, "")
	require.NoError(t, err)

	err = google_chat.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}

func Fuzz_GoogleChat(f *testing.F) {
	f.Add("", "error", "", "", []byte{}, []byte{})
	f.Add("", "info", "", "update", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		urlSuffix, severity, message, commitStatus string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		googlechat, err := NewGoogleChat(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), "")
		if err != nil {
			return
		}

		event := events.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}

		event.Metadata["commit_status"] = commitStatus
		event.Message = message
		event.Severity = severity

		_ = googlechat.Post(context.TODO(), event)
	})
}
