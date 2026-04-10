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
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/hashicorp/go-retryablehttp"
)

type Opsgenie struct {
	URL       string
	ProxyURL  string
	TLSConfig *tls.Config
	ApiKey    string
}

type OpsgenieAlert struct {
	Message     string            `json:"message"`
	Alias       string            `json:"alias,omitempty"`
	Description string            `json:"description"`
	Details     map[string]string `json:"details"`
}

func NewOpsgenie(hookURL string, proxyURL string, tlsConfig *tls.Config, token string) (*Opsgenie, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Opsgenie hook URL %s: '%w'", hookURL, err)
	}

	if token == "" {
		return nil, errors.New("empty Opsgenie apikey/token")
	}

	return &Opsgenie{
		URL:       hookURL,
		ProxyURL:  proxyURL,
		ApiKey:    token,
		TLSConfig: tlsConfig,
	}, nil
}

// Post opsgenie alert message
func (s *Opsgenie) Post(ctx context.Context, event eventv1.Event) error {
	var details = make(map[string]string)

	if event.Metadata != nil {
		details = event.Metadata
	}
	details["severity"] = event.Severity

	// Construct a stable alias for deduplication in Opsgenie.
	// The alias is derived from the involved object's kind, namespace,
	// name, and the event reason so that repeated alerts for the same
	// source are deduplicated while different reasons create separate alerts.
	alias := generateOpsgenieAlias(event)

	payload := OpsgenieAlert{
		Message:     event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
		Alias:       alias,
		Description: event.Message,
		Details:     details,
	}

	opts := []postOption{
		withRequestModifier(func(req *retryablehttp.Request) {
			req.Header.Set("Authorization", "GenieKey "+s.ApiKey)
		}),
	}
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

// generateOpsgenieAlias creates a stable, deterministic alias string from
// the event's involved object and reason. Opsgenie uses the alias field to
// deduplicate alerts — alerts with the same alias are treated as the same
// incident instead of creating new pages. The alias is a SHA-256 hash
// (truncated to 64 chars) to stay within Opsgenie's 512-char alias limit
// while remaining collision-resistant.
func generateOpsgenieAlias(event eventv1.Event) string {
	key := fmt.Sprintf("%s/%s/%s/%s",
		event.InvolvedObject.Kind,
		event.InvolvedObject.Namespace,
		event.InvolvedObject.Name,
		event.Reason,
	)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
	return hash[:64]
}
