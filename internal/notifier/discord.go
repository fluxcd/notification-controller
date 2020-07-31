/*
Copyright 2020 The Flux CD contributors.

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
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/fluxcd/pkg/recorder"
)

// Discord holds the hook URL
type Discord struct {
	URL      string
	Username string
	Channel  string
}

// NewDiscord validates the URL and returns a Discord object
func NewDiscord(hookURL string, username string, channel string) (*Discord, error) {
	webhook, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Discord hook URL %s", hookURL)
	}

	// use Slack formatting
	// https://birdie0.github.io/discord-webhooks-guide/other/slack_formatting.html
	if !strings.HasSuffix(hookURL, "/slack") {
		webhook.Path = path.Join(webhook.Path, "slack")
		hookURL = webhook.String()
	}

	if username == "" {
		return nil, errors.New("empty Discord username")
	}

	if channel == "" {
		return nil, errors.New("empty Discord channel")
	}

	return &Discord{
		Channel:  channel,
		URL:      hookURL,
		Username: username,
	}, nil
}

// Post Discord message
func (s *Discord) Post(event recorder.Event) error {
	payload := SlackPayload{
		Channel:  s.Channel,
		Username: s.Username,
	}
	if payload.Username == "" {
		payload.Username = event.ReportingController
	}

	color := "good"
	if event.Severity == recorder.EventSeverityError {
		color = "danger"
	}

	sfields := make([]SlackField, 0, len(event.Metadata))
	for k, v := range event.Metadata {
		sfields = append(sfields, SlackField{k, v, false})
	}

	a := SlackAttachment{
		Color:      color,
		AuthorName: fmt.Sprintf("%s.%s", event.InvolvedObject.Name, event.InvolvedObject.Namespace),
		Text:       event.Message,
		MrkdwnIn:   []string{"text"},
		Fields:     sfields,
	}

	payload.Attachments = []SlackAttachment{a}

	err := postMessage(s.URL, payload)
	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}

	return nil
}
