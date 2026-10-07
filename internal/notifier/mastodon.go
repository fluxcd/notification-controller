/*
Copyright 2026 The Flux authors

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
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/hashicorp/go-retryablehttp"
)

// mastodonStatusesPath is the endpoint for publishing a status.
// Reference: https://docs.joinmastodon.org/methods/statuses/#create
const mastodonStatusesPath = "/api/v1/statuses"

// mastodonMaxChars is the default status character limit of a Mastodon
// server. Statuses are truncated to this length as the limit cannot be
// discovered without an extra API call and exceeding it fails the post.
const mastodonMaxChars = 500

// Mastodon holds the server URL and OAuth access token
// for posting statuses to a Mastodon account.
type Mastodon struct {
	// URL is the fully resolved statuses endpoint of the server.
	URL       string
	ProxyURL  string
	Token     string
	TLSConfig *tls.Config
	// EventKey is sent as the Idempotency-Key header when set.
	EventKey string
}

// MastodonPayload is the JSON form accepted by the statuses endpoint.
// Visibility is intentionally not set so that statuses are posted with
// the default posting privacy configured on the account.
type MastodonPayload struct {
	Status string `json:"status"`
}

// NewMastodon validates the Mastodon server URL and returns a Mastodon
// object. The address may be the server root URL, in which case the
// statuses API path is appended. The event key is the identity of the
// event computed by the event server and is sent as the Idempotency-Key.
func NewMastodon(serverURL string, proxyURL string, tlsConfig *tls.Config, token string, eventKey string) (*Mastodon, error) {
	u, err := url.ParseRequestURI(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Mastodon server URL %s: '%w'", serverURL, err)
	}

	if token == "" {
		return nil, errors.New("empty Mastodon access token")
	}

	if !strings.HasSuffix(strings.TrimSuffix(u.Path, "/"), mastodonStatusesPath) {
		u.Path = strings.TrimSuffix(u.Path, "/") + mastodonStatusesPath
	}

	return &Mastodon{
		URL:       u.String(),
		ProxyURL:  proxyURL,
		Token:     token,
		TLSConfig: tlsConfig,
		EventKey:  eventKey,
	}, nil
}

// Post the event as a status on the Mastodon account owning the token.
func (m *Mastodon) Post(ctx context.Context, event eventv1.Event) error {
	emoji := "💫"
	if event.Severity == eventv1.EventSeverityError {
		emoji = "🚨"
	}

	heading := fmt.Sprintf("%s %s/%s.%s", emoji, strings.ToLower(event.InvolvedObject.Kind),
		event.InvolvedObject.Name, event.InvolvedObject.Namespace)

	var metadata strings.Builder
	for _, k := range slices.Sorted(maps.Keys(event.Metadata)) {
		v := event.Metadata[k]
		metadata.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	status := fmt.Sprintf("%s\n%s\n\n%s", heading, event.Message, metadata.String())
	status = strings.TrimSpace(status)
	if runes := []rune(status); len(runes) > mastodonMaxChars {
		status = string(runes[:mastodonMaxChars-1]) + "…"
	}

	payload := MastodonPayload{Status: status}

	// The Idempotency-Key header prevents a duplicate status when a retried
	// request succeeded but its response was lost. It carries the event key
	// the event server uses for rate limiting, so that an event has a single
	// identity across the controller. Mastodon keeps the key for one hour.
	opts := []postOption{
		withRequestModifier(func(req *retryablehttp.Request) {
			req.Header.Set("Authorization", "Bearer "+m.Token)
			if m.EventKey != "" {
				req.Header.Set("Idempotency-Key", m.EventKey)
			}
		}),
	}
	if m.ProxyURL != "" {
		opts = append(opts, withProxy(m.ProxyURL))
	}
	if m.TLSConfig != nil {
		opts = append(opts, withTLSConfig(m.TLSConfig))
	}

	if err := postMessage(ctx, m.URL, payload, opts...); err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}

	return nil
}
