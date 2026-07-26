/*
Copyright 2026 The Flux authors

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

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
)

func TestIncidentio_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.Header.Get("Authorization")).To(Equal("Bearer token"))
		g.Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload IncidentioPayload
		g.Expect(json.Unmarshal(b, &payload)).To(Succeed())

		g.Expect(payload.Title).To(Equal("gitrepository/webapp.gitops-system: reason"))
		g.Expect(payload.Status).To(Equal("resolved"))
		g.Expect(payload.DeduplicationKey).To(Equal("2f5c9d21-b62c-4a35-9b17-1b0f4c8e2d3a"))
		g.Expect(payload.Description).To(Equal("message"))
		g.Expect(payload.Metadata).To(HaveKeyWithValue("test", "metadata"))
		g.Expect(payload.Metadata).To(HaveKeyWithValue("severity", "info"))
		g.Expect(payload.Metadata).To(HaveKeyWithValue("reason", "reason"))
		g.Expect(payload.Metadata).To(HaveKeyWithValue("reportingController", "source-controller"))

		// incident.io responds with 202 Accepted
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	incidentio, err := NewIncidentio(ts.URL, "", nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.InvolvedObject.UID = "2f5c9d21-b62c-4a35-9b17-1b0f4c8e2d3a"
	err = incidentio.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestIncidentio_PostErrorSeverity(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload IncidentioPayload
		g.Expect(json.Unmarshal(b, &payload)).To(Succeed())

		g.Expect(payload.Status).To(Equal("firing"))
		g.Expect(payload.Metadata).To(HaveKeyWithValue("severity", "error"))

		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	incidentio, err := NewIncidentio(ts.URL, "", nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Severity = eventv1.EventSeverityError
	err = incidentio.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestIncidentio_PostSkipsProgressing(t *testing.T) {
	g := NewWithT(t)
	requested := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
	}))
	defer ts.Close()

	incidentio, err := NewIncidentio(ts.URL, "", nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Reason = meta.ProgressingReason
	err = incidentio.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(requested).To(BeFalse())
}

func TestNewIncidentio(t *testing.T) {
	g := NewWithT(t)

	_, err := NewIncidentio("invalid-url", "", nil, "token")
	g.Expect(err).To(MatchError(ContainSubstring("invalid incident.io alert source URL")))

	_, err = NewIncidentio("https://api.incident.io/v2/alert_events/http/some-id", "", nil, "")
	g.Expect(err).To(MatchError(ContainSubstring("empty incident.io API token")))
}
