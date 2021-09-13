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

// Slack holds the hook URL
type Slack struct {
	URL      string
	ProxyURL string
	Token    string
	Username string
	Channel  string
	CertPool *x509.CertPool
}

// SlackPayload holds the channel and attachments
type SlackPayload struct {
	Channel     string            `json:"channel"`
	Token       string            `json:"token,omitempty"`
	Username    string            `json:"username"`
	IconUrl     string            `json:"icon_url"`
	IconEmoji   string            `json:"icon_emoji"`
	Text        string            `json:"text,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment holds the markdown message body
type SlackAttachment struct {
	Color      string       `json:"color"`
	AuthorName string       `json:"author_name"`
	Text       string       `json:"text"`
	MrkdwnIn   []string     `json:"mrkdwn_in"`
	Fields     []SlackField `json:"fields"`
}

type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlack validates the Slack URL and returns a Slack object
func NewSlack(hookURL string, proxyURL string, token string, certPool *x509.CertPool, username string, channel string) (*Slack, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Slack hook URL %s", hookURL)
	}

	return &Slack{
		Channel:  channel,
		Username: username,
		URL:      hookURL,
		ProxyURL: proxyURL,
		Token:    token,
		CertPool: certPool,
	}, nil
}

// Post Slack message
func (s *Slack) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	payload := SlackPayload{
		Username: s.Username,
	}

	if s.Channel != "" {
		payload.Channel = s.Channel
	}

	if payload.Username == "" {
		payload.Username = event.ReportingController
	}

	color := "good"
	if event.Severity == events.EventSeverityError {
		color = "danger"
	}

	sfields := make([]SlackField, 0, len(event.Metadata))
	for k, v := range event.Metadata {
		sfields = append(sfields, SlackField{k, v, false})
	}

	a := SlackAttachment{
		Color:      color,
		AuthorName: fmt.Sprintf("%s/%s.%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.InvolvedObject.Namespace),
		Text:       event.Message,
		MrkdwnIn:   []string{"text"},
		Fields:     sfields,
	}

	payload.Attachments = []SlackAttachment{a}

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
