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

	"github.com/fluxcd/pkg/runtime/events"
	"github.com/stretchr/testify/require"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func TestAlertmanager_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload []AlertManagerAlert
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

	}))
	defer ts.Close()

	alertmanager, err := NewAlertmanager(ts.URL, "", nil)
	require.NoError(t, err)

	err = alertmanager.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}

func Fuzz_AlertManager(f *testing.F) {
	f.Add("update", "", "", []byte{}, []byte("{}"))
	f.Add("something", "", "else", []byte{}, []byte(""))

	f.Fuzz(func(t *testing.T,
		commitStatus, urlSuffix, summary string, seed, response []byte) {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		alertmanager, err := NewAlertmanager(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), "", &cert)
		if err != nil {
			return
		}

		event := events.Event{}

		// Try to fuzz the event object, but if it fails (not enough seed),
		// ignore it, as other inputs are also being used in this test.
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil && (commitStatus != "" || summary != "") {
			event.Metadata = map[string]string{
				"commit_status": commitStatus,
				"summary":       summary,
			}
		}

		_ = alertmanager.Post(context.TODO(), event)
	})
}
