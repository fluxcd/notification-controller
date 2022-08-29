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
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type requestOptFunc func(*retryablehttp.Request)

func postMessage(ctx context.Context, address, proxy string, certPool *x509.CertPool, payload interface{}, reqOpts ...requestOptFunc) error {
	httpClient := retryablehttp.NewClient()
	if certPool != nil {
		httpClient.HTTPClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		}
	}

	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return fmt.Errorf("unable to parse proxy URL '%s', error: %w", proxy, err)
		}
		var tlsConfig *tls.Config
		if certPool != nil {
			tlsConfig = &tls.Config{
				RootCAs: certPool,
			}
		}
		httpClient.HTTPClient.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: tlsConfig,
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
		}
	}

	httpClient.HTTPClient.Timeout = 15 * time.Second
	httpClient.RetryWaitMin = 2 * time.Second
	httpClient.RetryWaitMax = 30 * time.Second
	httpClient.RetryMax = 4
	httpClient.Logger = nil

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling notification payload failed: %w", err)
	}

	req, err := retryablehttp.NewRequest(http.MethodPost, address, data)
	if err != nil {
		return fmt.Errorf("failed to create a new request: %w", err)
	}
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, o := range reqOpts {
		o(req)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("unable to read response body, %s", err)
		}
		return fmt.Errorf("request failed with status code %d, %s", resp.StatusCode, string(b))
	}

	return nil
}
