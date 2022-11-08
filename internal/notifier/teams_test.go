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
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/stretchr/testify/require"
)

func TestTeams_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload = MSTeamsPayload{}
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		require.Equal(t, "gitrepository/webapp.gitops-system", payload.Sections[0].ActivitySubtitle)
		require.Equal(t, "metadata", payload.Sections[0].Facts[0].Value)
	}))
	defer ts.Close()

	teams, err := NewMSTeams(ts.URL, "", nil)
	require.NoError(t, err)

	err = teams.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}

func Fuzz_MSTeams(f *testing.F) {
	f.Add("token", "", "error", "", "", []byte{}, []byte{})
	f.Add("token", "", "info", "", "", []byte{}, []byte{})
	f.Add("token", "", "info", "", "update", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		token, urlSuffix, severity, message, commitStatus string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		teams, err := NewMSTeams(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), "", &cert)
		if err != nil {
			return
		}

		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}

		event.Metadata["commit_status"] = commitStatus
		event.Message = message
		event.Severity = severity

		_ = teams.Post(context.TODO(), event)
	})
}
