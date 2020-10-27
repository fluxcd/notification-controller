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
	"net/url"

	"github.com/fluxcd/pkg/recorder"
)

type Forwarder struct {
	URL      string
	ProxyURL string
}

func NewForwarder(hookURL string, proxyURL string) (*Forwarder, error) {
	if _, err := url.ParseRequestURI(hookURL); err != nil {
		return nil, fmt.Errorf("invalid Discord hook URL %s", hookURL)
	}

	return &Forwarder{
		URL:      hookURL,
		ProxyURL: proxyURL,
	}, nil
}

func (f *Forwarder) Post(event recorder.Event) error {
	err := postMessage(f.URL, f.ProxyURL, event)
	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
