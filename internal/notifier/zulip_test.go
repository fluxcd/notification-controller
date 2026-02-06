/*
Copyright 2025 The Flux authors

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

package notifier_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"

	"github.com/fluxcd/notification-controller/internal/notifier"
)

func TestNewZulip(t *testing.T) {
	t.Run("invalid endpoint", func(t *testing.T) {
		g := NewWithT(t)

		z, err := notifier.NewZulip(string([]byte{0x7f}), "general/announcements", "", nil, "user@example.com", "password")

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid Zulip endpoint URL"))
		g.Expect(z).To(BeNil())
	})

	t.Run("invalid channel format", func(t *testing.T) {
		g := NewWithT(t)

		z, err := notifier.NewZulip("https://zulip.example.com/api/v1/messages", "invalid_format", "", nil, "user@example.com", "password")

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("invalid Zulip channel format, expected <channel>/<topic>, got 'invalid_format'"))
		g.Expect(z).To(BeNil())
	})

	t.Run("valid channel format", func(t *testing.T) {
		g := NewWithT(t)

		z, err := notifier.NewZulip("https://zulip.example.com/api/v1/messages", "general/announcements", "", nil, "user@example.com", "password")

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(z).NotTo(BeNil())
	})
}

func TestZulip_Post(t *testing.T) {
	for _, tt := range []struct {
		name            string
		eventSeverity   string
		expectedContent string
	}{
		{
			name:            "info severity event",
			eventSeverity:   eventv1.EventSeverityInfo,
			expectedContent: "## Flux Status\n\nℹ️ Kustomization/default/test-ks\n\nTest event message\n\nMetadata:\n* `key1`: value1\n* `key2`: value2\n",
		},
		{
			name:            "error severity event",
			eventSeverity:   eventv1.EventSeverityError,
			expectedContent: "## Flux Status\n\n⚠️ Kustomization/default/test-ks\n\nTest event message\n\nMetadata:\n* `key1`: value1\n* `key2`: value2\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var req *http.Request

			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				req = r
				if err := r.ParseForm(); err != nil {
					t.Logf("failed to read request body: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}))
			t.Cleanup(s.Close)

			z, err := notifier.NewZulip(s.URL, "general/announcements", "", nil, "user", "pass")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(z).NotTo(BeNil())

			event := eventv1.Event{
				Severity: tt.eventSeverity,
				Message:  "Test event message",
				InvolvedObject: corev1.ObjectReference{
					Kind:      "Kustomization",
					Name:      "test-ks",
					Namespace: "default",
				},
				Metadata: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			}

			err = z.Post(context.Background(), event)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(req).NotTo(BeNil())
			g.Expect(req.Method).To(Equal(http.MethodPost))
			g.Expect(req.URL.Path).To(Equal("/api/v1/messages"))

			user, pass, ok := req.BasicAuth()
			g.Expect(ok).To(BeTrue())
			g.Expect(user).To(Equal("user"))
			g.Expect(pass).To(Equal("pass"))

			g.Expect(req.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))
			g.Expect(req.Form.Get("type")).To(Equal("stream"))
			g.Expect(req.Form.Get("to")).To(Equal("general"))
			g.Expect(req.Form.Get("topic")).To(Equal("announcements"))
			g.Expect(req.Form.Get("content")).To(Equal(tt.expectedContent))
		})
	}
}
