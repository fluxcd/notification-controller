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
	"fmt"
	"github.com/fluxcd/pkg/runtime/events"
	"net/url"

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
}

func NewForwarder(hookURL string, proxyURL string) (*Forwarder, error) {
	if _, err := url.ParseRequestURI(hookURL); err != nil {
		return nil, fmt.Errorf("invalid hook URL %s: %w", hookURL, err)
	}

	return &Forwarder{
		URL:      hookURL,
		ProxyURL: proxyURL,
	}, nil
}

func (f *Forwarder) Post(event events.Event) error {
	err := postMessage(f.URL, f.ProxyURL, event, func(req *retryablehttp.Request) {
		req.Header.Set(NotificationHeader, event.ReportingController)
	})

	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
