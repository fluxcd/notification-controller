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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/fluxcd/pkg/runtime/events"
	"github.com/getsentry/sentry-go"
)

// Sentry holds the client instance
type Sentry struct {
	Client *sentry.Client
}

// NewSentry creates a Sentry client from the provided Data Source Name (DSN)
func NewSentry(certPool *x509.CertPool, dsn string, environment string) (*Sentry, error) {
	tr := &http.Transport{}
	if certPool != nil {
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		}
	}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:           dsn,
		Environment:   environment,
		HTTPTransport: tr,
	})
	if err != nil {
		return nil, err
	}

	return &Sentry{
		Client: client,
	}, nil
}

// Post event to Sentry
func (s *Sentry) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	// Send event to Sentry
	s.Client.CaptureEvent(toSentryEvent(event), nil, nil)
	return nil
}

// Maps a controller-issued event to a Sentry event
func toSentryEvent(event events.Event) *sentry.Event {
	// Prepare Metadata
	extra := make(map[string]interface{}, len(event.Metadata))
	for k, v := range event.Metadata {
		extra[k] = v
	}

	// Construct event
	obj := event.InvolvedObject
	return &sentry.Event{
		Timestamp:   event.Timestamp.Time,
		Level:       sentry.Level(event.Severity),
		ServerName:  event.ReportingController,
		Transaction: fmt.Sprintf("%s: %s/%s", obj.Kind, obj.Namespace, obj.Name),
		Extra:       extra,
		Message:     event.Message,
	}
}
