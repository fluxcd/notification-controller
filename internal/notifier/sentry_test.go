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
	"errors"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/getsentry/sentry-go"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// captureTransport is a sentry.Transport that records the events it is asked
// to send, so tests can assert what the notifier captured.
type captureTransport struct {
	events []*sentry.Event
}

func (t *captureTransport) Configure(sentry.ClientOptions)        {}
func (t *captureTransport) SendEvent(event *sentry.Event)         { t.events = append(t.events, event) }
func (t *captureTransport) Flush(time.Duration) bool              { return true }
func (t *captureTransport) FlushWithContext(context.Context) bool { return true }
func (t *captureTransport) Close()                                {}

func TestNewSentry(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		environment string
		err         error
	}{
		{
			name:        "valid DSN",
			dsn:         "https://test@localhost/1",
			environment: "foo",
			err:         nil,
		},
		{
			name:        "empty DSN",
			dsn:         "",
			environment: "foo",
			err:         errors.New("DSN cannot be empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			n, err := NewSentry(t.Context(), &notifierOptions{URL: tt.dsn, Channel: tt.environment})
			if tt.err != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(Equal(tt.err))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				s := n.(*Sentry)
				g.Expect(s.Client.Options().Dsn).To(Equal(tt.dsn))
				g.Expect(s.Client.Options().Environment).To(Equal(tt.environment))
			}
		})
	}
}

func TestSentryPostSeverity(t *testing.T) {
	newEvent := func(severity string) eventv1.Event {
		return eventv1.Event{
			InvolvedObject: corev1.ObjectReference{
				Kind:      "GitRepository",
				Namespace: "flux-system",
				Name:      "test-app",
			},
			Severity:            severity,
			Timestamp:           metav1.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC),
			Message:             "message",
			ReportingController: "source-controller",
		}
	}

	tests := []struct {
		name         string
		severity     string
		wantCaptured bool
		wantType     string
	}{
		{name: "trace is sent as a trace", severity: eventv1.EventSeverityTrace, wantCaptured: true, wantType: "transaction"},
		{name: "info is sent as a trace", severity: eventv1.EventSeverityInfo, wantCaptured: true, wantType: "transaction"},
		{name: "error is sent as an event", severity: eventv1.EventSeverityError, wantCaptured: true, wantType: ""},
		{name: "unknown severity is not sent", severity: "warning", wantCaptured: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			transport := &captureTransport{}
			client, err := sentry.NewClient(sentry.ClientOptions{
				Dsn:              "https://test@localhost/1",
				Transport:        transport,
				SampleRate:       1,
				TracesSampleRate: 1,
			})
			g.Expect(err).ToNot(HaveOccurred())

			s := &Sentry{Client: client}
			g.Expect(s.Post(t.Context(), newEvent(tt.severity))).To(Succeed())
			client.Flush(time.Second)

			if !tt.wantCaptured {
				g.Expect(transport.events).To(BeEmpty())
				return
			}

			g.Expect(transport.events).To(HaveLen(1))
			g.Expect(transport.events[0].Type).To(Equal(tt.wantType))
			g.Expect(transport.events[0].Transaction).To(Equal("GitRepository: flux-system/test-app"))
		})
	}
}

func TestToSentryEvent(t *testing.T) {
	g := NewWithT(t)
	// Construct test event
	e := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "GitRepository",
			Namespace: "flux-system",
			Name:      "test-app",
		},
		Severity:  "info",
		Timestamp: metav1.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC),
		Message:   "message",
		Metadata: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		ReportingController: "source-controller",
	}

	// Map to Sentry event
	s := toSentryEvent(e)

	// Assertions
	g.Expect(s.Timestamp).To(Equal(time.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC)))
	g.Expect(s.Level).To(Equal(sentry.LevelInfo))
	g.Expect(s.ServerName).To(Equal("source-controller"))
	g.Expect(s.Transaction).To(Equal("GitRepository: flux-system/test-app"))
	g.Expect(s.Contexts).To(Equal(map[string]sentry.Context{
		"metadata": {
			"key1": "val1",
			"key2": "val2",
		},
	}))
	g.Expect(s.Message).To(Equal("message"))
}

func TestToSentrySpan(t *testing.T) {
	g := NewWithT(t)
	// Construct test event
	e := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "GitRepository",
			Namespace: "flux-system",
			Name:      "test-app",
		},
		Severity:  "info",
		Timestamp: metav1.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC),
		Message:   "message",
		Metadata: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		ReportingController: "source-controller",
	}

	// Map to Sentry event
	s := eventToSpan(e)

	// Assertions
	g.Expect(s.Timestamp).To(Equal(time.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC)))
	g.Expect(s.Type).To(Equal("transaction"))
	g.Expect(s.Tags).To(Equal(map[string]string{
		"flux_involved_object_kind":      e.InvolvedObject.Kind,
		"flux_involved_object_name":      e.InvolvedObject.Name,
		"flux_involved_object_namespace": e.InvolvedObject.Namespace,
		"flux_reason":                    e.Reason,
		"flux_reporting_controller":      e.ReportingController,
		"flux_reporting_instance":        e.ReportingInstance,
		"key1":                           "val1",
		"key2":                           "val2",
	}))
	g.Expect(s.Message).To(Equal("message"))
}
