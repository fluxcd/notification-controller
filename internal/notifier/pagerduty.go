/*
Copyright 2023 The Flux authors

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
	"fmt"
	"net/url"
	"time"

	"github.com/PagerDuty/go-pagerduty"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
)

type PagerDuty struct {
	Endpoint   string
	RoutingKey string
	ProxyURL   string
	TLSConfig  *tls.Config
}

func NewPagerDuty(endpoint string, proxyURL string, tlsConfig *tls.Config, routingKey string) (*PagerDuty, error) {
	URL, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid PagerDuty endpoint URL %q: '%w'", endpoint, err)
	}
	return &PagerDuty{
		Endpoint:   URL.Scheme + "://" + URL.Host,
		RoutingKey: routingKey,
		ProxyURL:   proxyURL,
		TLSConfig:  tlsConfig,
	}, nil
}

func (p *PagerDuty) Post(ctx context.Context, event eventv1.Event) error {
	// skip commit status updates and progressing events (we want success or failure)
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) || event.HasReason(meta.ProgressingReason) {
		return nil
	}

	var opts []postOption
	if p.ProxyURL != "" {
		opts = append(opts, withProxy(p.ProxyURL))
	}
	if p.TLSConfig != nil {
		opts = append(opts, withTLSConfig(p.TLSConfig))
	}

	if err := postMessage(
		ctx,
		p.Endpoint+"/v2/enqueue",
		toPagerDutyV2Event(event, p.RoutingKey),
		opts...,
	); err != nil {
		return fmt.Errorf("failed sending event: %w", err)
	}

	// Send a change event for info events
	if event.Severity == eventv1.EventSeverityInfo {
		if err := postMessage(
			ctx,
			p.Endpoint+"/v2/change/enqueue",
			toPagerDutyChangeEvent(event, p.RoutingKey),
			opts...,
		); err != nil {
			return fmt.Errorf("failed sending change event: %w", err)
		}
	}

	return nil
}

func toPagerDutyV2Event(event eventv1.Event, routingKey string) pagerduty.V2Event {
	name, desc := formatNameAndDescription(event)
	// Send resolve just in case an existing incident is open
	e := pagerduty.V2Event{
		RoutingKey: routingKey,
		Action:     "resolve",
		DedupKey:   string(event.InvolvedObject.UID),
	}
	// Trigger an incident for errors
	if event.Severity == eventv1.EventSeverityError {
		e.Action = "trigger"
		e.Payload = &pagerduty.V2Payload{
			Summary:   desc + ": " + name,
			Source:    "Flux " + event.ReportingController,
			Severity:  toPagerDutySeverity(event.Severity),
			Timestamp: event.Timestamp.Format(time.RFC3339),
			Component: event.InvolvedObject.Name,
			Group:     event.InvolvedObject.Kind,
			Details: map[string]interface{}{
				"message":  event.Message,
				"metadata": event.Metadata,
			},
		}
	}
	return e
}

func toPagerDutyChangeEvent(event eventv1.Event, routingKey string) pagerduty.ChangeEvent {
	name, desc := formatNameAndDescription(event)
	ce := pagerduty.ChangeEvent{
		RoutingKey: routingKey,
		Payload: pagerduty.ChangeEventPayload{
			Summary:   desc + ": " + name,
			Source:    "Flux " + event.ReportingController,
			Timestamp: event.Timestamp.Format(time.RFC3339),
			CustomDetails: map[string]interface{}{
				"message":  event.Message,
				"metadata": event.Metadata,
			},
		},
	}
	return ce
}

func toPagerDutySeverity(severity string) string {
	switch severity {
	case eventv1.EventSeverityError:
	case eventv1.EventSeverityInfo:
		return severity
	case eventv1.EventSeverityTrace:
		return "info"
	}
	return "error"
}
