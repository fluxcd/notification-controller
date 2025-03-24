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

	"github.com/stretchr/testify/require"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestSlack_Post(t *testing.T) {
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

	slack, err := NewSlack(ts.URL, "", "", nil, "", "test")
	require.NoError(t, err)

	err = slack.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}

func TestSlack_PostUpdate(t *testing.T) {
	slack, err := NewSlack("http://localhost", "", "", nil, "", "test")
	require.NoError(t, err)

	event := testEvent()
	event.Metadata[eventv1.MetaCommitStatusKey] = eventv1.MetaCommitStatusUpdateValue
	err = slack.Post(context.TODO(), event)
	require.NoError(t, err)
}

func TestSlack_ValidateResponse(t *testing.T) {
	body := []byte(`{
  "ok": true
}`)
	err := validateSlackResponse(http.StatusOK, body)
	require.NoError(t, err)

	body = []byte(`{
  "ok": false,
  "error": "too_many_attachments"
}`)
	err = validateSlackResponse(http.StatusOK, body)
	require.ErrorContains(t, err, "Slack responded with error: too_many_attachments")
}
