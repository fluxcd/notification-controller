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

	"github.com/fluxcd/pkg/runtime/events"

	"github.com/stretchr/testify/require"
)

func TestForwarder_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		require.Equal(t, "source-controller", r.Header.Get("gotk-component"))
		require.Equal(t, "token", r.Header.Get("Authorization"))
		var payload = events.Event{}
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)
		require.Equal(t, "webapp", payload.InvolvedObject.Name)
		require.Equal(t, "metadata", payload.Metadata["test"])
	}))
	defer ts.Close()

	headers := make(map[string]string)
	headers["Authorization"] = "token"
	forwarder, err := NewForwarder(ts.URL, "", headers, nil)
	require.NoError(t, err)

	err = forwarder.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}
