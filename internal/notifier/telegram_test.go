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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTelegram_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload = WebexPayload{}
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)
	}))
	defer ts.Close()

	telegram, err := NewTelegram("channel", "token")
	require.NoError(t, err)

	telegram.send = func(url, message string) error {
		require.Equal(t, "telegram://token@telegram?channels=channel&parseMode=markDownv2", url)
		require.Equal(t, "*💫 gitrepository/webapp/gitops\\-system*\nmessage\n\\- *test*: metadata\n\\- *kubernetes\\.io/somekey*: some\\.value\n", message)
		return nil
	}

	ev := testEvent()
	ev.Metadata["kubernetes.io/somekey"] = "some.value"
	err = telegram.Post(context.TODO(), ev)
	require.NoError(t, err)
}
