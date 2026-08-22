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

// Zoom holds the incoming webhook URL and verification token
// for a Zoom Team Chat Incoming Webhook chatbot connection.
type Zoom struct {
	URL       string
	ProxyURL  string
	Token     string
	TLSConfig *tls.Config
}

// ZoomPayload is the rich message format accepted by the
// Incoming Webhook endpoint when called with `?format=full`.
type ZoomPayload struct {
	Content ZoomContent `json:"content"`
}

type ZoomContent struct {
	Head ZoomHead       `json:"head"`
	Body []ZoomBodyItem `json:"body"`
}

type ZoomHead struct {
	Text    string       `json:"text"`
	SubHead *ZoomSubHead `json:"sub_head,omitempty"`
}

type ZoomSubHead struct {
	Text string `json:"text"`
}

type ZoomBodyItem struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	Items []ZoomField `json:"items,omitempty"`
}

type ZoomField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// NewZoom validates the Zoom incoming webhook URL and returns a Zoom object
func NewZoom(hookURL string, proxyURL string, tlsConfig *tls.Config, token string) (*Zoom, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Zoom incoming webhook URL %s: '%w'", hookURL, err)
	}

	if token == "" {
		return nil, errors.New("empty Zoom verification token")
	}

	return &Zoom{
		URL:       hookURL,
		ProxyURL:  proxyURL,
		Token:     token,
		TLSConfig: tlsConfig,
	}, nil
}

// Post Zoom Team Chat message
func (s *Zoom) Post(ctx context.Context, event eventv1.Event) error {
	// Request the rich message format unless the URL already pins one.
	u, err := url.ParseRequestURI(s.URL)
	if err != nil {
		return fmt.Errorf("invalid Zoom incoming webhook URL: %w", err)
	}
	q := u.Query()
	if q.Get("format") == "" {
		q.Set("format", "full")
		u.RawQuery = q.Encode()
	}

	objName := fmt.Sprintf("%s/%s.%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.InvolvedObject.Namespace)

	body := []ZoomBodyItem{
		{
			Type: "message",
			Text: event.Message,
		},
	}

	if len(event.Metadata) > 0 {
		fields := make([]ZoomField, 0, len(event.Metadata))
		for _, k := range slices.Sorted(maps.Keys(event.Metadata)) {
			v := event.Metadata[k]
			fields = append(fields, ZoomField{
				Key:   k,
				Value: v,
			})
		}
		body = append(body, ZoomBodyItem{
			Type:  "fields",
			Items: fields,
		})
	}

	payload := ZoomPayload{
		Content: ZoomContent{
			Head: ZoomHead{
				Text: objName,
				SubHead: &ZoomSubHead{
					Text: event.Severity,
				},
			},
			Body: body,
		},
	}

	opts := []postOption{
		// The Incoming Webhook expects the raw verification token
		// in the Authorization header, without a scheme prefix.
		withRequestModifier(func(req *retryablehttp.Request) {
			req.Header.Set("Authorization", s.Token)
		}),
	}
	if s.ProxyURL != "" {
		opts = append(opts, withProxy(s.ProxyURL))
	}
	if s.TLSConfig != nil {
		opts = append(opts, withTLSConfig(s.TLSConfig))
	}

	if err := postMessage(ctx, u.String(), payload, opts...); err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}

	return nil
}
