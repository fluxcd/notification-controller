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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fluxcd/pkg/apis/event/v1beta1"
	. "github.com/onsi/gomega"
)

func TestOpsgenie_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload OpsgenieAlert
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

	}))
	defer ts.Close()

	tests := []struct {
		name  string
		event func() v1beta1.Event
	}{
		{
			name:  "test event",
			event: testEvent,
		},
		{
			name: "test event with empty metadata",
			event: func() v1beta1.Event {
				events := testEvent()
				events.Metadata = nil
				return events
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			opsgenie, err := NewOpsgenie(ts.URL, "", nil, "token", "")
			g.Expect(err).ToNot(HaveOccurred())

			err = opsgenie.Post(context.TODO(), tt.event())
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}

func TestOpsgenie_PostAlias(t *testing.T) {
	var receivedPayload OpsgenieAlert
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.Unmarshal(b, &receivedPayload)
	}))
	defer ts.Close()

	providerUID := "test-provider-uid-123"

	tests := []struct {
		name          string
		event         func() v1beta1.Event
		expectedAlias string
	}{
		{
			name:  "alias includes provider UID for cluster uniqueness",
			event: testEvent,
			expectedAlias: fmt.Sprintf("%x",
				sha256.Sum256([]byte("test-provider-uid-123/GitRepository/gitops-system/webapp/reason")))[:64],
		},
		{
			name: "alias is stable for same event",
			event: func() v1beta1.Event {
				e := testEvent()
				e.Message = "different message should not change alias"
				return e
			},
			expectedAlias: fmt.Sprintf("%x",
				sha256.Sum256([]byte("test-provider-uid-123/GitRepository/gitops-system/webapp/reason")))[:64],
		},
		{
			name: "alias differs for different reason",
			event: func() v1beta1.Event {
				e := testEvent()
				e.Reason = "HealthCheckFailed"
				return e
			},
			expectedAlias: fmt.Sprintf("%x",
				sha256.Sum256([]byte("test-provider-uid-123/GitRepository/gitops-system/webapp/HealthCheckFailed")))[:64],
		},
		{
			name: "alias differs for different namespace",
			event: func() v1beta1.Event {
				e := testEvent()
				e.InvolvedObject.Namespace = "production"
				return e
			},
			expectedAlias: fmt.Sprintf("%x",
				sha256.Sum256([]byte("test-provider-uid-123/GitRepository/production/webapp/reason")))[:64],
		},
		{
			name: "alias with empty metadata",
			event: func() v1beta1.Event {
				e := testEvent()
				e.Metadata = nil
				return e
			},
			expectedAlias: fmt.Sprintf("%x",
				sha256.Sum256([]byte("test-provider-uid-123/GitRepository/gitops-system/webapp/reason")))[:64],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			opsgenie, err := NewOpsgenie(ts.URL, "", nil, "token", providerUID)
			g.Expect(err).ToNot(HaveOccurred())

			err = opsgenie.Post(context.TODO(), tt.event())
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(receivedPayload.Alias).To(Equal(tt.expectedAlias))
			g.Expect(receivedPayload.Alias).ToNot(BeEmpty())
		})
	}
}

func TestGenerateOpsgenieAlias(t *testing.T) {
	g := NewWithT(t)
	event := testEvent()
	providerUID := "test-uid"

	// Alias should be deterministic
	alias1 := generateOpsgenieAlias(providerUID, event)
	alias2 := generateOpsgenieAlias(providerUID, event)
	g.Expect(alias1).To(Equal(alias2))

	// Alias should be 64 chars (hex-encoded SHA-256 truncated)
	g.Expect(alias1).To(HaveLen(64))

	// Different reason should produce different alias
	event2 := testEvent()
	event2.Reason = "DifferentReason"
	alias3 := generateOpsgenieAlias(providerUID, event2)
	g.Expect(alias1).ToNot(Equal(alias3))

	// Different provider UID should produce different alias
	alias4 := generateOpsgenieAlias("different-uid", event)
	g.Expect(alias1).ToNot(Equal(alias4))
}
