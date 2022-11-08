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
	"strings"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscord_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload = SlackPayload{}
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)
		require.Equal(t, "gitrepository/webapp.gitops-system", payload.Attachments[0].AuthorName)
		require.Equal(t, "metadata", payload.Attachments[0].Fields[0].Value)
	}))
	defer ts.Close()

	discord, err := NewDiscord(ts.URL, "", "test", "test")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(discord.URL, "/slack"))

	err = discord.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}

func Fuzz_Discord(f *testing.F) {
	f.Add("username", "channel", "/slack", "info", "update", []byte{}, []byte("{}"))
	f.Add("", "channel", "", "error", "", []byte{}, []byte(""))

	f.Fuzz(func(t *testing.T,
		username, channel, urlSuffix, severity, commitStatus string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			w.Write(response)
			r.Body.Close()
		}))
		defer ts.Close()

		discord, err := NewDiscord(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), "", username, channel)
		if err != nil {
			return
		}

		event := eventv1.Event{}
		// Try to fuzz the event object, but if it fails (not enough seed),
		// ignore it, as other inputs are also being used in this test.
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}

		event.Metadata["commit_status"] = commitStatus
		event.Severity = severity

		_ = discord.Post(context.TODO(), event)
	})
}
