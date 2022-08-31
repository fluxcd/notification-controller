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
	"crypto/x509"
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxcd/pkg/runtime/events"
	"github.com/hashicorp/go-retryablehttp"
)

type Grafana struct {
	URL      string
	Token    string
	ProxyURL string
	CertPool *x509.CertPool
	Username string
	Password string
}

// GraphiteAnnotation represents a Grafana API annotation in Graphite format
type GraphitePayload struct {
	When int64    `json:"when"` //optional unix timestamp (ms)
	Text string   `json:"text"`
	Tags []string `json:"tags,omitempty"`
}

// NewGrafana validates the Grafana URL and returns a Grafana object
func NewGrafana(URL string, proxyURL string, token string, certPool *x509.CertPool, username string, password string) (*Grafana, error) {
	_, err := url.ParseRequestURI(URL)
	if err != nil {
		return nil, fmt.Errorf("invalid Grafana URL %s", URL)
	}

	return &Grafana{
		URL:      URL,
		ProxyURL: proxyURL,
		Token:    token,
		CertPool: certPool,
		Username: username,
		Password: password,
	}, nil
}

// Post annotation
func (g *Grafana) Post(ctx context.Context, event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	sfields := make([]string, 0, len(event.Metadata))
	// add tag to filter on grafana
	sfields = append(sfields, "flux", event.ReportingController)
	for k, v := range event.Metadata {
		sfields = append(sfields, fmt.Sprintf("%s: %s", k, v))
	}
	payload := GraphitePayload{
		When: event.Timestamp.Unix(),
		Text: fmt.Sprintf("%s/%s.%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.InvolvedObject.Namespace),
		Tags: sfields,
	}

	err := postMessage(ctx, g.URL, g.ProxyURL, g.CertPool, payload, func(request *retryablehttp.Request) {
		if (g.Username != "" && g.Password != "") && g.Token == "" {
			request.Header.Add("Authorization", "Basic "+basicAuth(g.Username, g.Password))
		}
		if g.Token != "" {
			request.Header.Add("Authorization", "Bearer "+g.Token)
		}
	})
	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
