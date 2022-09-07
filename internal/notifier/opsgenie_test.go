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
	"crypto/x509"
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

func TestOpsgenie_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload OpsgenieAlert
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

	}))
	defer ts.Close()

	opsgenie, err := NewOpsgenie(ts.URL, "", nil, "token")
	require.NoError(t, err)

	err = opsgenie.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}

func Fuzz_OpsGenie(f *testing.F) {
	f.Add("token", "", "", "error", "", []byte{}, []byte{})
	f.Add("token", "", "update", "info", "", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		token, urlSuffix, commitStatus, severity, message string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		opsgenie, err := NewOpsgenie(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), "", &cert, token)
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

		_ = opsgenie.Post(context.TODO(), event)
	})
}
