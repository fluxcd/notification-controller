/*
Copyright 2025 The Flux authors

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

type Zulip struct {
	endpoint  string
	channel   string
	topic     string
	proxyURL  string
	tlsConfig *tls.Config
	username  string
	password  string
}

func NewZulip(endpoint, channel, proxyURL string, tlsConfig *tls.Config, username, password string) (*Zulip, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid Zulip endpoint URL: %w", err)
	}
	u.Path = "/api/v1/messages"
	endpoint = u.String()

	s := strings.SplitN(channel, "/", 2)
	if len(s) != 2 || s[0] == "" || s[1] == "" {
		return nil, fmt.Errorf("invalid Zulip channel format, expected <channel>/<topic>, got '%s'", channel)
	}
	channel = s[0]
	topic := s[1]

	return &Zulip{
		endpoint:  endpoint,
		channel:   channel,
		topic:     topic,
		proxyURL:  proxyURL,
		tlsConfig: tlsConfig,
		username:  username,
		password:  password,
	}, nil
}

func (z *Zulip) Post(ctx context.Context, event eventv1.Event) error {
	const contentType = "application/x-www-form-urlencoded"

	content := formatMarkdownPost(&event)

	payload := []byte(url.Values{
		"type":    {"stream"},
		"to":      {z.channel},
		"topic":   {z.topic},
		"content": {content},
	}.Encode())

	return postMessage(ctx, z.endpoint, payload,
		withProxy(z.proxyURL),
		withTLSConfig(z.tlsConfig),
		withBasicAuth(z.username, z.password),
		withContentType(contentType))
}
