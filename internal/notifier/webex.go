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
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxcd/pkg/runtime/events"
)

// Webex holds the hook URL
type Webex struct {
	URL      string
	ProxyURL string
}

// WebexPayload holds the message text
type WebexPayload struct {
	Text     string `json:"text,omitempty"`
	Markdown string `json:"markdown,omitempty"`
}

// NewWebex validates the Webex URL and returns a Webex object
func NewWebex(hookURL, proxyURL string) (*Webex, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Webex hook URL %s", hookURL)
	}

	return &Webex{
		URL:      hookURL,
		ProxyURL: proxyURL,
	}, nil
}

// Post Webex message
func (s *Webex) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	objName := fmt.Sprintf("%s/%s.%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	markdown := fmt.Sprintf("> **NAME** = %s | **MESSAGE** = %s", objName, event.Message)

	if len(event.Metadata) > 0 {
		markdown += " | **METADATA** ="
		for k, v := range event.Metadata {
			markdown += fmt.Sprintf(" **%s**: %s", k, v)
		}
	}

	payload := WebexPayload{
		Text:     "",
		Markdown: markdown,
	}

	if err := postMessage(s.URL, s.ProxyURL, payload); err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
