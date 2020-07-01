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
	"github.com/fluxcd/pkg/recorder"
	"net/url"
)

// Rocket holds the hook URL
type Rocket struct {
	URL      string
	Username string
	Channel  string
}

// NewRocket validates the Rocket URL and returns a Rocket object
func NewRocket(hookURL string, username string, channel string) (*Rocket, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Rocket hook URL %s", hookURL)
	}

	if username == "" {
		return nil, errors.New("empty Rocket username")
	}

	if channel == "" {
		return nil, errors.New("empty Rocket channel")
	}

	return &Rocket{
		Channel:  channel,
		URL:      hookURL,
		Username: username,
	}, nil
}

// Post Rocket message
func (s *Rocket) Post(event recorder.Event) error {
	payload := SlackPayload{
		Channel:   s.Channel,
		Username:  s.Username,
		IconEmoji: ":rocket:",
	}

	color := "#0076D7"
	if event.Severity == recorder.EventSeverityError {
		color = "#FF0000"
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
