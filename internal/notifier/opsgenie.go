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

	payload := OpsgenieAlert{
		Message:     event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
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
