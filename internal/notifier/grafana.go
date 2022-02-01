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
	"crypto/x509"
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxcd/pkg/runtime/events"
	"github.com/hashicorp/go-retryablehttp"
)

// Discord holds the hook URL
type Grafana struct {
	URL      string
	Token    string
	ProxyURL string
	CertPool *x509.CertPool
}

// GraphiteAnnotation represents a Grafana API annotation in Graphite format
type GraphitePayload struct {
	//What string   `json:"what"` //optional
	When int64    `json:"when"` //optional unix timestamp (ms)
	Text string   `json:"text"`
	Tags []string `json:"tags,omitempty"`
}

// NewGrafana validates the Grafana URL and returns a Grafana object
func NewGrafana(URL string, proxyURL string, token string, certPool *x509.CertPool) (*Grafana, error) {
	_, err := url.ParseRequestURI(URL)
	if err != nil {
		return nil, fmt.Errorf("invalid Grafana URL %s", URL)
	}

	return &Grafana{
		URL:      URL,
		ProxyURL: proxyURL,
		Token:    token,
		CertPool: certPool,
	}, nil
}

// Post annotation
func (s *Grafana) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	sfields := make([]string, 0, len(event.Metadata))
	sfields = append(sfields, "flux")
	sfields = append(sfields, event.ReportingController)
	for k, v := range event.Metadata {
		sfields = append(sfields, fmt.Sprintf("%s: %s", k, v))
	}
	// add tag to filter on grafana

	payload := GraphitePayload{
		When: event.Timestamp.Unix(),
		Text: fmt.Sprintf("%s/%s.%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.InvolvedObject.Namespace),
		Tags: sfields,
	}

	err := postMessage(s.URL, s.ProxyURL, s.CertPool, payload, func(request *retryablehttp.Request) {
		if s.Token != "" {
			request.Header.Add("Authorization", "Bearer "+s.Token)
		}
	})
	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
