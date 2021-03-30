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
	"crypto/sha256"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fluxcd/pkg/runtime/events"
	"github.com/stretchr/testify/require"
)

func TestForwarder_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)

		require.Equal(t, "source-controller", r.Header.Get("gotk-component"))

		payload := events.Event{}
		require.NoError(t, json.Unmarshal(b, &payload))

		require.NoError(t, err)
		require.Equal(t, "webapp", payload.InvolvedObject.Name)
		require.Equal(t, "metadata", payload.Metadata["test"])
		require.Empty(t, r.Header.Get(SignatureHeader))
	}))
	defer ts.Close()

	forwarder, err := NewForwarder(ts.URL, "", "")
	require.NoError(t, err)

	err = forwarder.Post(testEvent())
	require.NoError(t, forwarder.Post(testEvent()))
}

func TestForwarder_with_SigningSecret(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)

		payload := events.Event{}
		require.NoError(t, json.Unmarshal(b, &payload))

		s, err := bytesSignature(sha256.New, "testing", b)
		require.NoError(t, err)
		require.Equal(t, r.Header.Get(SignatureHeader),
			"sha256="+s)
	}))
	defer ts.Close()

	forwarder, err := NewForwarder(ts.URL, "", "testing")
	require.NoError(t, err)

	require.NoError(t, forwarder.Post(testEvent()))
}
