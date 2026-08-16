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
	"crypto/sha256"
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
	URL        string
	ProxyURL   string
	Token      string
	Visibility string
	TLSConfig  *tls.Config
}

// MastodonPayload is the JSON form accepted by the statuses endpoint.
type MastodonPayload struct {
	Status     string `json:"status"`
	Visibility string `json:"visibility,omitempty"`
}

// NewMastodon validates the Mastodon server URL and returns a Mastodon
// object. The address may be the server root URL, in which case the
// statuses API path is appended. An optional `visibility` query parameter
// (public, unlisted, private) overrides the app's default status visibility.
func NewMastodon(serverURL string, proxyURL string, tlsConfig *tls.Config, token string) (*Mastodon, error) {
	u, err := url.ParseRequestURI(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Mastodon server URL %s: '%w'", serverURL, err)
	}

	if token == "" {
		return nil, errors.New("empty Mastodon access token")
	}

	// The visibility is carried as a query parameter of the address
	// because the Provider API has no dedicated field for it.
	q := u.Query()
	visibility := q.Get("visibility")
	q.Del("visibility")
	u.RawQuery = q.Encode()

	if !strings.HasSuffix(strings.TrimSuffix(u.Path, "/"), mastodonStatusesPath) {
		u.Path = strings.TrimSuffix(u.Path, "/") + mastodonStatusesPath
	}

	return &Mastodon{
		URL:        u.String(),
		ProxyURL:   proxyURL,
		Token:      token,
		Visibility: visibility,
		TLSConfig:  tlsConfig,
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

	payload := MastodonPayload{
		Status:     status,
		Visibility: m.Visibility,
	}

	// The Idempotency-Key header prevents a duplicate status when a retried
	// request succeeded but its response was lost. The event timestamp keeps
	// the key unique across recurring events of the same object.
	idempotencyKey := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%s",
		event.InvolvedObject.UID, event.Reason, event.Timestamp.UTC().String(), status)))

	opts := []postOption{
		withRequestModifier(func(req *retryablehttp.Request) {
			req.Header.Set("Authorization", "Bearer "+m.Token)
			req.Header.Set("Idempotency-Key", fmt.Sprintf("%x", idempotencyKey))
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
