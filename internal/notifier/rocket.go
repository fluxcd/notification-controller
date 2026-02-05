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
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

// Rocket holds the hook URL
type Rocket struct {
	URL       string
	ProxyURL  string
	Username  string
	Channel   string
	TLSConfig *tls.Config
}

// NewRocket validates the Rocket URL and returns a Rocket object
func NewRocket(hookURL string, proxyURL string, tlsConfig *tls.Config, username string, channel string) (*Rocket, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Rocket hook URL %s: '%w'", hookURL, err)
	}

	return &Rocket{
		Channel:   channel,
		URL:       hookURL,
		ProxyURL:  proxyURL,
		Username:  username,
		TLSConfig: tlsConfig,
	}, nil
}

// Post Rocket message
func (s *Rocket) Post(ctx context.Context, event eventv1.Event) error {
	payload := SlackPayload{
		Channel:  s.Channel,
		Username: s.Username,
	}

	color := "#0076D7"
	if event.Severity == eventv1.EventSeverityError {
		color = "#FF0000"
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

	var opts []postOption
	if s.ProxyURL != "" {
		opts = append(opts, withProxy(s.ProxyURL))
	}
	if s.TLSConfig != nil {
		opts = append(opts, withTLSConfig(s.TLSConfig))
	}

	if err := postMessage(ctx, s.URL, payload, opts...); err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
