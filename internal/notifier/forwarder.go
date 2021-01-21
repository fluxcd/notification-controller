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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"net/url"

	"github.com/fluxcd/pkg/runtime/events"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	// NotificationHeader is a header sent to identify requests from the
	// notification controller.
	NotificationHeader = "gotk-component"
	// SignatureHeader is a header with the signature of the body of the request
	// using a shared-secret mechanism.
	SignatureHeader = "X-Signature"
)

// Forwarder is an implementation of the notification Interface that posts the
// body as an HTTP request using an optional proxy.
type Forwarder struct {
	URL           string
	ProxyURL      string
	SigningSecret string
}

// NewForwarder creates and returns a generic event forwarder pre-configured for
// the endpoint and optionally with a proxy URL.
func NewForwarder(hookURL, proxyURL, secret string) (*Forwarder, error) {
	if _, err := url.ParseRequestURI(hookURL); err != nil {
		return nil, fmt.Errorf("invalid hook URL %s: %w", hookURL, err)
	}

	return &Forwarder{
		URL:           hookURL,
		ProxyURL:      proxyURL,
		SigningSecret: secret,
	}, nil
}

func (f *Forwarder) Post(event events.Event) error {
	opts := []requestOptFunc{func(req *retryablehttp.Request) error {
		req.Header.Set(NotificationHeader, event.ReportingController)
		return nil
	}}
	if f.SigningSecret != "" {
		opts = append(opts, requestSigner("sha256", f.SigningSecret, sha256.New))
	}
	err := postMessage(f.URL, f.ProxyURL, event, opts...)
	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}

func bytesSignature(h func() hash.Hash, secret string, b []byte) (string, error) {
	hm := hmac.New(h, []byte(secret))
	_, err := hm.Write(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate hmac for body: %w", err)
	}
	return hex.EncodeToString(hm.Sum(nil)), nil
}

func requestSigner(kind, secret string, h func() hash.Hash) requestOptFunc {
	return func(req *retryablehttp.Request) error {
		b, err := req.BodyBytes()
		if err != nil {
			return fmt.Errorf("failed to get body when signing request: %w", err)
		}
		s, err := bytesSignature(h, secret, b)
		if err != nil {
			return err
		}
		req.Header.Set(SignatureHeader, fmt.Sprintf("%s=%s", kind, s))
		return nil
	}
}
