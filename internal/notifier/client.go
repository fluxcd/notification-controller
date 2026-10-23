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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	maxErrorBodyBytes    = 4 * 1024
	maxResponseBodyBytes = 1024 * 1024
)

var (
	retryMu      sync.RWMutex
	retryMax     = 4
	retryWaitMin = 2 * time.Second
	retryWaitMax = 30 * time.Second
)

// SetRetryBackoff overrides retry waits for newly created clients, for tests only.
// Call it from a non-parallel parent test before starting parallel subtests,
// and register the returned restore function with the parent's Cleanup.
func SetRetryBackoff(minWait, maxWait time.Duration) (restore func()) {
	retryMu.Lock()
	previousMin, previousMax := retryWaitMin, retryWaitMax
	retryWaitMin, retryWaitMax = minWait, maxWait
	retryMu.Unlock()

	return func() {
		retryMu.Lock()
		retryWaitMin, retryWaitMax = previousMin, previousMax
		retryMu.Unlock()
	}
}

type postOptions struct {
	proxy             *url.URL
	tlsConfig         *tls.Config
	contentType       string
	username          string
	password          string
	requestModifier   func(*retryablehttp.Request)
	responseValidator func(*http.Response) error
}

type postOption func(*postOptions)

func postMessage(ctx context.Context, address string, payload any, opts ...postOption) error {
	options := &postOptions{
		// Default validateResponse accepts any 2xx status (including 204 No Content).
		responseValidator: func(resp *http.Response) error {
			s := resp.StatusCode
			if 200 <= s && s < 300 {
				return nil
			}

			err := fmt.Errorf("request failed with status code %d", s)

			b, bodyErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes+1))
			if bodyErr != nil {
				return fmt.Errorf("%w: unable to read response body: %w", err, bodyErr)
			}

			if len(b) > maxErrorBodyBytes {
				return fmt.Errorf("%w: %s... (truncated)", err, strings.TrimSpace(string(b[:maxErrorBodyBytes])))
			}
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(b)))
		},
	}

	for _, o := range opts {
		o(options)
	}

	httpClient := newHTTPClient(options)

	contentType := options.contentType
	var data []byte
	switch contentType {
	case "":
		contentType = "application/json"
		var err error
		data, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshalling notification payload failed: %w", err)
		}
	default:
		var ok bool
		data, ok = payload.([]byte)
		if !ok {
			return fmt.Errorf("notification payload must be []byte for content type %q, got %T", contentType, payload)
		}
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, address, data)
	if err != nil {
		return fmt.Errorf("failed to create a new request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	if options.username != "" || options.password != "" {
		req.SetBasicAuth(options.username, options.password)
	}

	if options.requestModifier != nil {
		options.requestModifier(req)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if err := options.responseValidator(resp); err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	return nil
}

func withProxy(proxy *url.URL) postOption {
	return func(opts *postOptions) {
		opts.proxy = proxy
	}
}

func withTLSConfig(tlsConfig *tls.Config) postOption {
	return func(opts *postOptions) {
		opts.tlsConfig = tlsConfig
	}
}

func withContentType(contentType string) postOption {
	return func(opts *postOptions) {
		opts.contentType = contentType
	}
}

func withBasicAuth(username, password string) postOption {
	return func(opts *postOptions) {
		opts.username = username
		opts.password = password
	}
}

func withRequestModifier(reqModifier func(*retryablehttp.Request)) postOption {
	return func(opts *postOptions) {
		opts.requestModifier = reqModifier
	}
}

func withResponseValidator(respValidator func(resp *http.Response) error) postOption {
	return func(opts *postOptions) {
		opts.responseValidator = respValidator
	}
}

// newTransport clones the default HTTP transport, preserving its timeouts,
// connection pooling and environment proxy unless an explicit proxy is provided.
// A non-nil TLS configuration is cloned so HTTP/2 setup does not mutate the
// caller's configuration.
func newTransport(proxyURL *url.URL, tlsConfig *tls.Config) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig.Clone()
	}
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return transport
}

func newHTTPClient(opts *postOptions) *retryablehttp.Client {
	httpClient := retryablehttp.NewClient()

	transport := httpClient.HTTPClient.Transport.(*http.Transport)

	if opts.tlsConfig != nil {
		transport.TLSClientConfig = opts.tlsConfig
	}

	if opts.proxy != nil {
		transport.Proxy = http.ProxyURL(opts.proxy)
	}

	// Disable the timeout for the HTTP client,
	// as we set the provider timeout on the context.
	httpClient.HTTPClient.Timeout = 0

	retryMu.RLock()
	httpClient.RetryWaitMin = retryWaitMin
	httpClient.RetryWaitMax = retryWaitMax
	httpClient.RetryMax = retryMax
	retryMu.RUnlock()
	httpClient.CheckRetry = retryablehttp.ErrorPropagatedRetryPolicy
	httpClient.Logger = nil

	return httpClient
}
