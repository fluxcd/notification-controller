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
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
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
			g := NewWithT(t)
			_, err := NewForwarder("http://example.org", "", nil, nil, tt.hmacKey)
			if tt.err {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
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
			g := NewWithT(t)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, err := io.ReadAll(r.Body)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(r.Header.Get("gotk-component")).To(Equal("source-controller"))
				g.Expect(r.Header.Get("Authorization")).To(Equal("token"))
				if tt.hmacHeader == "" {
					sigHdrVal, ok := r.Header["X-Signature"]
					if tt.xSigHeader == "" {
						g.Expect(ok).To(BeFalse(), "expected signature header to be absent but it was present")
					} else {
						g.Expect(sigHdrVal).To(Equal([]string{tt.xSigHeader}))
					}
				} else {
					g.Expect(r.Header.Get("X-Signature")).To(Equal(tt.hmacHeader))
				}
				var payload = eventv1.Event{}
				err = json.Unmarshal(b, &payload)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(payload.InvolvedObject.Name).To(Equal("webapp"))
				g.Expect(payload.Metadata["test"]).To(Equal("metadata"))
			}))
			defer ts.Close()

			headers := make(map[string]string)
			headers["Authorization"] = "token"
			if tt.xSigHeader != "" {
				headers["X-Signature"] = tt.xSigHeader
			}
			forwarder, err := NewForwarder(ts.URL, "", headers, nil, tt.hmacKey)
			g.Expect(err).ToNot(HaveOccurred())

			ev := testEvent()
			ev.Timestamp = metav1.NewTime(time.Unix(1664520029, 0))
			err = forwarder.Post(context.TODO(), ev)
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}
