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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebex_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)

		var payload = WebexPayload{}
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)
		require.Empty(t, payload.Text)
		require.Equal(t, "> **NAME** = gitrepository/webapp.gitops-system | **MESSAGE** = message | **METADATA** = **test**: metadata", payload.Markdown)
	}))
	defer ts.Close()

	webex, err := NewWebex(ts.URL, "", nil)
	require.NoError(t, err)

	err = webex.Post(testEvent())
	require.NoError(t, err)
}

func TestWebex_PostUpdate(t *testing.T) {
	webex, err := NewWebex("http://localhost", "", nil)
	require.NoError(t, err)

	event := testEvent()
	event.Metadata["commit_status"] = "update"
	err = webex.Post(event)
	require.NoError(t, err)
}
