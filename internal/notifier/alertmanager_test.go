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
)

func TestNewAlertmanager(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		g := NewWithT(t)
		am, err := NewAlertmanager("https://alertmanager.example.com/api/v2/alerts", "", nil, "", "", "")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(am.URL).To(Equal("https://alertmanager.example.com/api/v2/alerts"))
	})

	t.Run("invalid URL", func(t *testing.T) {
		g := NewWithT(t)
		_, err := NewAlertmanager("not a url", "", nil, "", "", "")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid Alertmanager URL"))
	})
}

func TestAlertmanager_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload []AlertManagerAlert
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload).To(HaveLen(1))
		alert := payload[0]
		g.Expect(alert.Status).To(Equal("firing"))
		g.Expect(alert.Labels["alertname"]).To(Equal("FluxGitRepositoryReason"))
		g.Expect(alert.Labels["severity"]).To(Equal("info"))
		g.Expect(alert.Labels["reason"]).To(Equal("reason"))
		g.Expect(alert.Labels["kind"]).To(Equal("GitRepository"))
		g.Expect(alert.Labels["name"]).To(Equal("webapp"))
		g.Expect(alert.Labels["namespace"]).To(Equal("gitops-system"))
		g.Expect(alert.Labels["reportingcontroller"]).To(Equal("source-controller"))
		g.Expect(alert.Annotations["message"]).To(Equal("message"))
	}))
	defer ts.Close()

	alertmanager, err := NewAlertmanager(ts.URL, "", nil, "", "", "")
	g.Expect(err).ToNot(HaveOccurred())

	err = alertmanager.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestAlertmanager_PostWithBearerToken(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.Header.Get("Authorization")).To(Equal("Bearer test-token"))
	}))
	defer ts.Close()

	alertmanager, err := NewAlertmanager(ts.URL, "", nil, "test-token", "", "")
	g.Expect(err).ToNot(HaveOccurred())

	err = alertmanager.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestAlertmanager_PostWithBasicAuth(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		g.Expect(ok).To(BeTrue())
		g.Expect(username).To(Equal("admin"))
		g.Expect(password).To(Equal("secret"))
	}))
	defer ts.Close()

	alertmanager, err := NewAlertmanager(ts.URL, "", nil, "", "admin", "secret")
	g.Expect(err).ToNot(HaveOccurred())

	err = alertmanager.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestAlertmanager_PostWithSummaryMetadata(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload []AlertManagerAlert
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		alert := payload[0]
		g.Expect(alert.Annotations["summary"]).To(Equal("deployment failed"))
		// summary should not appear as a label
		_, hasSummaryLabel := alert.Labels["summary"]
		g.Expect(hasSummaryLabel).To(BeFalse())
	}))
	defer ts.Close()

	alertmanager, err := NewAlertmanager(ts.URL, "", nil, "", "", "")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Metadata["summary"] = "deployment failed"
	err = alertmanager.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestAlertmanager_PostServerError(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	alertmanager, err := NewAlertmanager(ts.URL, "", nil, "", "", "")
	g.Expect(err).ToNot(HaveOccurred())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = alertmanager.Post(ctx, testEvent())
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("postMessage failed"))
}

func TestAlertManagerTime_MarshalJSON(t *testing.T) {
	g := NewWithT(t)
	ts := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	amt := AlertManagerTime(ts)

	b, err := json.Marshal(amt)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(b)).To(Equal(`"2026-03-01T12:00:00Z"`))
}

func TestAlertManagerTime_UnmarshalJSON(t *testing.T) {
	g := NewWithT(t)
	var amt AlertManagerTime
	err := json.Unmarshal([]byte(`"2026-03-01T12:00:00Z"`), &amt)
	g.Expect(err).ToNot(HaveOccurred())

	expected := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	g.Expect(time.Time(amt)).To(Equal(expected))
}
