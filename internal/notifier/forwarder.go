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
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/url"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"

	"github.com/hashicorp/go-retryablehttp"
)

// NotificationHeader is a header sent to identify requests from the
// notification controller.
const NotificationHeader = "gotk-component"

// Forwarder is an implementation of the notification Interface that posts the
// body as an HTTP request using an optional proxy.
type Forwarder struct {
	URL      string
	ProxyURL string
	Headers  map[string]string
	CertPool *x509.CertPool
	HMACKey  []byte
}

func NewForwarder(hookURL string, proxyURL string, headers map[string]string, certPool *x509.CertPool, hmacKey []byte) (*Forwarder, error) {
	if _, err := url.ParseRequestURI(hookURL); err != nil {
		return nil, fmt.Errorf("invalid hook URL %s: %w", hookURL, err)
	}

	if hmacKey != nil && len(hmacKey) == 0 {
		return nil, fmt.Errorf("HMAC key is empty")
	}

	return &Forwarder{
		URL:      hookURL,
		ProxyURL: proxyURL,
		Headers:  headers,
		CertPool: certPool,
		HMACKey:  hmacKey,
	}, nil
}

func sign(payload, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(payload)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (f *Forwarder) Post(ctx context.Context, event eventv1.Event) error {
	var sig string
	if len(f.HMACKey) != 0 {
		eventJSON, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed marshalling event: %w", err)
		}
		sig = fmt.Sprintf("sha256=%s", sign(eventJSON, f.HMACKey))
	}
	err := postMessage(ctx, f.URL, f.ProxyURL, f.CertPool, event, func(req *retryablehttp.Request) {
		req.Header.Set(NotificationHeader, event.ReportingController)
		for key, val := range f.Headers {
			req.Header.Set(key, val)
		}
		if sig != "" {
			req.Header.Set("X-Signature", sig)
		}
	})

	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
