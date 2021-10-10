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
)

// MS Teams holds the incoming webhook URL
type MSTeams struct {
	URL      string
	ProxyURL string
	CertPool *x509.CertPool
}

// MSTeamsPayload holds the message card data
type MSTeamsPayload struct {
	Type       string           `json:"@type"`
	Context    string           `json:"@context"`
	ThemeColor string           `json:"themeColor"`
	Summary    string           `json:"summary"`
	Sections   []MSTeamsSection `json:"sections"`
}

// MSTeamsSection holds the canary analysis result
type MSTeamsSection struct {
	ActivityTitle    string         `json:"activityTitle"`
	ActivitySubtitle string         `json:"activitySubtitle"`
	Facts            []MSTeamsField `json:"facts"`
}

type MSTeamsField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NewMSTeams validates the MS Teams URL and returns a MSTeams object
func NewMSTeams(hookURL string, proxyURL string, certPool *x509.CertPool) (*MSTeams, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid MS Teams webhook URL %s", hookURL)
	}

	return &MSTeams{
		URL:      hookURL,
		ProxyURL: proxyURL,
		CertPool: certPool,
	}, nil
}

// Post MS Teams message
func (s *MSTeams) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	facts := make([]MSTeamsField, 0, len(event.Metadata))
	for k, v := range event.Metadata {
		facts = append(facts, MSTeamsField{
			Name:  k,
			Value: v,
		})
	}

	objName := fmt.Sprintf("%s/%s.%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	payload := MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: "0076D7",
		Summary:    objName,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    event.Message,
				ActivitySubtitle: objName,
				Facts:            facts,
			},
		},
	}

	if event.Severity == events.EventSeverityError {
		payload.ThemeColor = "FF0000"
	}

	err := postMessage(s.URL, s.ProxyURL, s.CertPool, payload)
	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}

	return nil
}
