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
	"slices"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

const (
	msTeamsSchemaDeprecatedConnector = iota
	msTeamsSchemaAdaptiveCard

	// msAdaptiveCardVersion is the version of the MS Adaptive Card schema.
	// MS Teams currently supports only up to version 1.4:
	// https://community.powerplatform.com/forums/thread/details/?threadid=edde0a5d-e995-4ba3-96dc-2120fe51a4d0
	msAdaptiveCardVersion = "1.4"
)

// MS Teams holds the incoming webhook URL
type MSTeams struct {
	URL      string
	ProxyURL string
	CertPool *x509.CertPool
	Schema   int
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

// The Adaptice Card payload structures below reflect this documentation:
// https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/connectors-using?tabs=cURL%2Ctext1#send-adaptive-cards-using-an-incoming-webhook

type msAdaptiveCardMessage struct {
	Type        string                     `json:"type"`
	Attachments []msAdaptiveCardAttachment `json:"attachments"`
}

type msAdaptiveCardAttachment struct {
	ContentType string                `json:"contentType"`
	Content     msAdaptiveCardContent `json:"content"`
}

type msAdaptiveCardContent struct {
	Schema  string                      `json:"$schema"`
	Type    string                      `json:"type"`
	Version string                      `json:"version"`
	Body    []msAdaptiveCardBodyElement `json:"body"`
	MSTeams msAdaptiveCardMSTeams       `json:"msteams"`
}

type msAdaptiveCardBodyElement struct {
	Type string `json:"type"`

	*msAdaptiveCardContainer `json:",inline"`
	*msAdaptiveCardTextBlock `json:",inline"`
	*msAdaptiveCardFactSet   `json:",inline"`
}

type msAdaptiveCardContainer struct {
	Items []msAdaptiveCardBodyElement `json:"items,omitempty"`
}

type msAdaptiveCardMSTeams struct {
	Width string `json:"width,omitempty"`
}

type msAdaptiveCardTextBlock struct {
	Text   string `json:"text,omitempty"`
	Size   string `json:"size,omitempty"`
	Weight string `json:"weight,omitempty"`
	Color  string `json:"color,omitempty"`
	Wrap   bool   `json:"wrap,omitempty"`
}

type msAdaptiveCardFactSet struct {
	Facts []msAdaptiveCardFact `json:"facts,omitempty"`
}

type msAdaptiveCardFact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

// NewMSTeams validates the MS Teams URL and returns a MSTeams object
func NewMSTeams(hookURL string, proxyURL string, certPool *x509.CertPool) (*MSTeams, error) {
	u, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid MS Teams webhook URL %s: '%w'", hookURL, err)
	}

	provider := &MSTeams{
		URL:      hookURL,
		ProxyURL: proxyURL,
		CertPool: certPool,
		Schema:   msTeamsSchemaAdaptiveCard,
	}

	// Check if the webhook URL is the deprecated connector and update the schema accordingly.
	if strings.HasSuffix(strings.Split(u.Host, ":")[0], ".webhook.office.com") {
		provider.Schema = msTeamsSchemaDeprecatedConnector
	}

	return provider, nil
}

// Post MS Teams message
func (s *MSTeams) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	objName := fmt.Sprintf("%s/%s.%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.InvolvedObject.Namespace)

	var payload any
	switch s.Schema {
	case msTeamsSchemaDeprecatedConnector:
		payload = buildMSTeamsDeprecatedConnectorPayload(&event, objName)
	case msTeamsSchemaAdaptiveCard:
		payload = buildMSTeamsAdaptiveCardPayload(&event, objName)
	default:
		payload = buildMSTeamsAdaptiveCardPayload(&event, objName)
	}

	var opts []postOption
	if s.ProxyURL != "" {
		opts = append(opts, withProxy(s.ProxyURL))
	}
	if s.CertPool != nil {
		opts = append(opts, withCertPool(s.CertPool))
	}

	if err := postMessage(ctx, s.URL, payload, opts...); err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}

	return nil
}

func buildMSTeamsDeprecatedConnectorPayload(event *eventv1.Event, objName string) *MSTeamsPayload {
	facts := make([]MSTeamsField, 0, len(event.Metadata))
	for k, v := range event.Metadata {
		facts = append(facts, MSTeamsField{
			Name:  k,
			Value: v,
		})
	}

	payload := &MSTeamsPayload{
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

	if event.Severity == eventv1.EventSeverityError {
		payload.ThemeColor = "FF0000"
	}

	return payload
}

func buildMSTeamsAdaptiveCardPayload(event *eventv1.Event, objName string) *msAdaptiveCardMessage {
	// Prepare message, add red color to error messages.
	message := &msAdaptiveCardTextBlock{
		Text: event.Message,
		Wrap: true,
	}
	if event.Severity == eventv1.EventSeverityError {
		message.Color = "attention"
	}

	// Put "summary" first, then sort the rest of the metadata by key.
	facts := make([]msAdaptiveCardFact, 0, len(event.Metadata))
	const summaryKey = "summary"
	if summary, ok := event.Metadata[summaryKey]; ok {
		facts = append(facts, msAdaptiveCardFact{
			Title: summaryKey,
			Value: summary,
		})
	}
	metadataFirstIndex := len(facts)
	for k, v := range event.Metadata {
		if k == summaryKey {
			continue
		}
		facts = append(facts, msAdaptiveCardFact{
			Title: k,
			Value: v,
		})
	}
	slices.SortFunc(facts[metadataFirstIndex:], func(a, b msAdaptiveCardFact) int {
		return strings.Compare(a.Title, b.Title)
	})

	// The card below was built with help from https://adaptivecards.io/designer using the Microsoft Teams host app.
	payload := &msAdaptiveCardMessage{
		Type: "message",
		Attachments: []msAdaptiveCardAttachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				Content: msAdaptiveCardContent{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: msAdaptiveCardVersion,
					MSTeams: msAdaptiveCardMSTeams{
						Width: "Full",
					},
					Body: []msAdaptiveCardBodyElement{
						{
							Type: "Container",
							msAdaptiveCardContainer: &msAdaptiveCardContainer{
								Items: []msAdaptiveCardBodyElement{
									{
										Type: "TextBlock",
										msAdaptiveCardTextBlock: &msAdaptiveCardTextBlock{
											Text:   objName,
											Size:   "large",
											Weight: "bolder",
											Wrap:   true,
										},
									},
									{
										Type:                    "TextBlock",
										msAdaptiveCardTextBlock: message,
									},
									{
										Type: "FactSet",
										msAdaptiveCardFactSet: &msAdaptiveCardFactSet{
											Facts: facts,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return payload
}
