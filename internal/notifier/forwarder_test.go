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
	"time"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/fluxcd/pkg/runtime/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

func TestForwarder_New(t *testing.T) {
	tests := []struct {
		name    string
		hmacKey []byte
		err     bool
	}{
		{
			name:    "nil HMAC key passes",
			hmacKey: nil,
			err:     false,
		},
		{
			name:    "empty HMAC key fails",
			hmacKey: []byte{},
			err:     true,
		},
		{
			name:    "happy path with HMAC key from empty string",
			hmacKey: []byte(""),
			err:     true,
		},
		{
			name:    "non-empty HMAC key adds signature header",
			hmacKey: []byte("7152fed34dd6149a7c75a276c510da27cb6f82b0"),
			err:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewForwarder("http://example.org", "", nil, nil, tt.hmacKey)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestForwarder_Post(t *testing.T) {
	tests := []struct {
		name       string
		hmacKey    []byte
		hmacHeader string
		xSigHeader string
	}{
		{
			name: "happy path with nil HMAC key",
		},
		{
			name:       "preset X-Signature header should persist",
			xSigHeader: "should be preserved",
		},
		{
			name:       "non-empty HMAC key adds signature header",
			hmacKey:    []byte("7152fed34dd6149a7c75a276c510da27cb6f82b0"),
			hmacHeader: "sha256=65b018549b1254e7226d1c08f9567ee45bc9de0fc4e7b1a40253f9a018b08be7",
			xSigHeader: "should be overwritten with actual signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				require.Equal(t, "source-controller", r.Header.Get("gotk-component"))
				require.Equal(t, "token", r.Header.Get("Authorization"))
				if tt.hmacHeader == "" {
					sigHdrVal, ok := r.Header["X-Signature"]
					if tt.xSigHeader == "" {
						require.Equal(t, false, ok, "expected signature header to be absent but it was present")
					} else {
						require.Equal(t, []string{tt.xSigHeader}, sigHdrVal)
					}
				} else {
					require.Equal(t, tt.hmacHeader, r.Header.Get("X-Signature"))
				}
				var payload = events.Event{}
				err = json.Unmarshal(b, &payload)
				require.NoError(t, err)
				require.Equal(t, "webapp", payload.InvolvedObject.Name)
				require.Equal(t, "metadata", payload.Metadata["test"])
			}))
			defer ts.Close()

			headers := make(map[string]string)
			headers["Authorization"] = "token"
			if tt.xSigHeader != "" {
				headers["X-Signature"] = tt.xSigHeader
			}
			forwarder, err := NewForwarder(ts.URL, "", headers, nil, tt.hmacKey)
			require.NoError(t, err)

			ev := testEvent()
			ev.Timestamp = metav1.NewTime(time.Unix(1664520029, 0))
			err = forwarder.Post(context.TODO(), ev)
			require.NoError(t, err)
		})
	}
}

func Fuzz_Forwarder(f *testing.F) {
	f.Add("", []byte{}, []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		urlSuffix string, seed, response, hmacKey []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		header := make(map[string]string)
		_ = fuzz.NewConsumer(seed).FuzzMap(&header)

		forwarder, err := NewForwarder(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), "", header, &cert, hmacKey)
		if err != nil {
			return
		}

		event := events.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		_ = forwarder.Post(context.TODO(), event)
	})
}
