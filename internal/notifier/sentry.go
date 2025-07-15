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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/getsentry/sentry-go"
)

// Sentry holds the client instance
type Sentry struct {
	Client *sentry.Client
}

// NewSentry creates a Sentry client from the provided Data Source Name (DSN)
func NewSentry(certPool *x509.CertPool, dsn string, environment string) (*Sentry, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DSN cannot be empty")
	}

	tr := &http.Transport{}
	if certPool != nil {
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		}
	}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		HTTPTransport:    tr,
		TracesSampleRate: 1,
	})
	if err != nil {
		return nil, err
	}

	return &Sentry{
		Client: client,
	}, nil
}

// Post event to Sentry
func (s *Sentry) Post(ctx context.Context, event eventv1.Event) error {
	var sev *sentry.Event
	// Send event to Sentry
	switch event.Severity {
	case eventv1.EventSeverityInfo:
		// Info is sent as a trace
		sev = eventToSpan(event)
	case eventv1.EventSeverityError:
		// Errors are sent as normal events
		sev = toSentryEvent(event)
	}
	s.Client.CaptureEvent(sev, nil, nil)

	return nil
}

// Convert a controller event to a Sentry trace
// Sentry traces work slightly different compared to normal events, they don't cause
// alerts by default and are saved differently.
// They are shown in a dashobard with graphs, so they can be used to check if and how often
// flux tasks are running
func eventToSpan(event eventv1.Event) *sentry.Event {
	obj := event.InvolvedObject

	// Sadly you can't create spans on specific clients, they are always auto-generated
	// from the context, and the client saved within
	span := sentry.StartSpan(context.Background(), "event")
	// TODO: Maybe change the tag names?
	span.SetTag("flux_involved_object_kind", obj.Kind)
	span.SetTag("flux_involved_object_namespace", obj.Namespace)
	span.SetTag("flux_involved_object_name", obj.Name)
	span.SetTag("flux_reporting_controller", event.ReportingController)
	span.SetTag("flux_reporting_instance", event.ReportingInstance)
	span.SetTag("flux_reason", event.Reason)
	span.StartTime = event.Timestamp.Time
	span.EndTime = event.Timestamp.Time

	for k, v := range event.Metadata {
		span.SetTag(k, v)
	}

	// So because the sentry-go sdk has no way to send transactions
	// with an explicit client, we have to do it ourselves
	return &sentry.Event{
		Type:        "transaction",
		Transaction: eventSummary(event),
		Message:     event.Message,
		Contexts: map[string]sentry.Context{
			"trace": sentry.TraceContext{
				TraceID:      span.TraceID,
				SpanID:       span.SpanID,
				ParentSpanID: span.ParentSpanID,
				Op:           span.Op,
				Description:  span.Description,
				Status:       span.Status,
			}.Map(),
		},
		Tags:      span.Tags,
		Extra:     span.Data,
		Timestamp: span.EndTime,
		StartTime: span.StartTime,
		Spans:     []*sentry.Span{span},
	}
}

func eventSummary(event eventv1.Event) string {
	obj := event.InvolvedObject
	return fmt.Sprintf("%s: %s/%s", obj.Kind, obj.Namespace, obj.Name)
}

// Maps a controller-issued event to a Sentry event
func toSentryEvent(event eventv1.Event) *sentry.Event {
	// Prepare Metadata
	extra := make(map[string]interface{}, len(event.Metadata))
	for k, v := range event.Metadata {
		extra[k] = v
	}

	// Construct event
	return &sentry.Event{
		Timestamp:   event.Timestamp.Time,
		Level:       sentry.Level(event.Severity),
		ServerName:  event.ReportingController,
		Transaction: eventSummary(event),
		Extra:       extra,
		Message:     event.Message,
	}
}
