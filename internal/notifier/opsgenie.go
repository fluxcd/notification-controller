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
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"

	"github.com/fluxcd/pkg/runtime/events"
	"github.com/hashicorp/go-retryablehttp"
)

type Opsgenie struct {
	URL      string
	ProxyURL string
	CertPool *x509.CertPool
	ApiKey   string
}

type OpsgenieAlert struct {
	Message     string            `json:"message"`
	Description string            `json:"description"`
	Details     map[string]string `json:"details"`
}

func NewOpsgenie(hookURL string, proxyURL string, certPool *x509.CertPool, token string) (*Opsgenie, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Opsgenie hook URL %s: '%w'", hookURL, err)
	}

	if token == "" {
		return nil, errors.New("empty Opsgenie apikey/token")
	}

	return &Opsgenie{
		URL:      hookURL,
		ProxyURL: proxyURL,
		CertPool: certPool,
		ApiKey:   token,
	}, nil
}

// Post opsgenie alert message
func (s *Opsgenie) Post(ctx context.Context, event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	payload := OpsgenieAlert{
		Message:     event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
		Description: event.Message,
		Details:     event.Metadata,
	}

	err := postMessage(s.URL, s.ProxyURL, s.CertPool, payload, func(req *retryablehttp.Request) {
		req.Header.Set("Authorization", "GenieKey "+s.ApiKey)
	})

	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
