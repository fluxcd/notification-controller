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
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/hashicorp/go-retryablehttp"
)

// Incidentio holds the endpoint URL of an incident.io HTTP alert source
// and its API token.
type Incidentio struct {
	URL       string
	ProxyURL  string
	Token     string
	TLSConfig *tls.Config
}

// IncidentioPayload is the alert event format accepted by the incident.io
// HTTP alert source endpoint (POST /v2/alert_events/http/{alert_source_config_id}).
type IncidentioPayload struct {
	Title            string            `json:"title"`
	Status           string            `json:"status"`
	DeduplicationKey string            `json:"deduplication_key"`
	Description      string            `json:"description,omitempty"`
	SourceURL        string            `json:"source_url,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// NewIncidentio validates the alert source endpoint URL and token
// and returns an Incidentio object.
func NewIncidentio(endpointURL string, proxyURL string, tlsConfig *tls.Config, token string) (*Incidentio, error) {
	_, err := url.ParseRequestURI(endpointURL)
	if err != nil {
		return nil, fmt.Errorf("invalid incident.io alert source URL %s: '%w'", endpointURL, err)
	}

	if token == "" {
		return nil, errors.New("empty incident.io API token")
	}

	return &Incidentio{
		URL:       endpointURL,
		ProxyURL:  proxyURL,
		Token:     token,
		TLSConfig: tlsConfig,
	}, nil
}

// Post event to the incident.io HTTP alert source. Error events fire an
// alert, any other severity resolves it. The involved object UID is used
// as the deduplication key so that a recovery event resolves the alert
// previously fired for the same object, and objects with the same
// kind/namespace/name on different clusters never share a key.
func (i *Incidentio) Post(ctx context.Context, event eventv1.Event) error {
	// Skip progressing events, we want either a terminal failure or a recovery.
	if event.HasReason(meta.ProgressingReason) {
		return nil
	}

	obj := event.InvolvedObject
	kind := strings.ToLower(obj.Kind)
	objName := fmt.Sprintf("%s/%s.%s", kind, obj.Name, obj.Namespace)

	status := "resolved"
	if event.Severity == eventv1.EventSeverityError {
		status = "firing"
	}

	metadata := make(map[string]string, len(event.Metadata)+3)
	for k, v := range event.Metadata {
		metadata[k] = v
	}
	metadata["severity"] = event.Severity
	metadata["reason"] = event.Reason
	if event.ReportingController != "" {
		metadata["reportingController"] = event.ReportingController
	}

	payload := IncidentioPayload{
		Title:            fmt.Sprintf("%s: %s", objName, event.Reason),
		Status:           status,
		DeduplicationKey: string(obj.UID),
		Description:      event.Message,
		Metadata:         metadata,
	}

	opts := []postOption{
		withRequestModifier(func(req *retryablehttp.Request) {
			req.Header.Set("Authorization", "Bearer "+i.Token)
		}),
	}
	if i.ProxyURL != "" {
		opts = append(opts, withProxy(i.ProxyURL))
	}
	if i.TLSConfig != nil {
		opts = append(opts, withTLSConfig(i.TLSConfig))
	}

	if err := postMessage(ctx, i.URL, payload, opts...); err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}

	return nil
}
