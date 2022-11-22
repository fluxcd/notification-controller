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

func TestRocket_Post(t *testing.T) {
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

	rocket, err := NewRocket(ts.URL, "", nil, "test", "test")
	require.NoError(t, err)

	err = rocket.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}

func Fuzz_Rocket(f *testing.F) {
	f.Add("user", "channel", "", "error", "", "", []byte{}, []byte{})
	f.Add("user", "channel", "", "error", "update", "", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		username, channel, urlSuffix, severity, commitStatus, message string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		rocket, err := NewRocket(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), "", &cert, username, channel)
		if err != nil {
			return
		}

		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}

		event.Metadata["commit_status"] = commitStatus
		event.Severity = severity
		event.Message = message

		_ = rocket.Post(context.TODO(), event)
	})
}
