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
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type Alertmanager struct {
	URL       string
	ProxyURL  string
	TLSConfig *tls.Config
	Token     string
	Username  string
	Password  string
}

type AlertManagerAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`

	StartsAt AlertManagerTime `json:"startsAt"`
	EndsAt   AlertManagerTime `json:"endsAt,omitempty"`
}

// AlertManagerTime takes care of representing time.Time as RFC3339.
// See https://prometheus.io/docs/alerting/0.27/clients/
type AlertManagerTime time.Time

func (a AlertManagerTime) String() string {
	return time.Time(a).Format(time.RFC3339)
}

func (a AlertManagerTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *AlertManagerTime) UnmarshalJSON(jsonRepr []byte) error {
	var serializedTime string
	if err := json.Unmarshal(jsonRepr, &serializedTime); err != nil {
		return err
	}

	t, err := time.Parse(time.RFC3339, serializedTime)
	if err != nil {
		return err
	}

	*a = AlertManagerTime(t)
	return nil
}

func NewAlertmanager(hookURL string, proxyURL string, tlsConfig *tls.Config, token, user, pass string) (*Alertmanager, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Alertmanager URL %s: '%w'", hookURL, err)
	}

	return &Alertmanager{
		URL:       hookURL,
		ProxyURL:  proxyURL,
		Token:     token,
		Username:  user,
		Password:  pass,
		TLSConfig: tlsConfig,
	}, nil
}

func (s *Alertmanager) Post(ctx context.Context, event eventv1.Event) error {
	annotations := make(map[string]string)
	annotations["message"] = event.Message

	_, ok := event.Metadata["summary"]
	if ok {
		annotations["summary"] = event.Metadata["summary"]
		delete(event.Metadata, "summary")
	}

	var labels = make(map[string]string)
	if event.Metadata != nil {
		labels = event.Metadata
	}
	labels["alertname"] = "Flux" + event.InvolvedObject.Kind + cases.Title(language.Und).String(event.Reason)
	labels["severity"] = event.Severity
	labels["reason"] = event.Reason

	labels["kind"] = event.InvolvedObject.Kind
	labels["name"] = event.InvolvedObject.Name
	labels["namespace"] = event.InvolvedObject.Namespace
	labels["reportingcontroller"] = event.ReportingController

	// The best reasonable `endsAt` value would be multiplying
	// InvolvedObject's reconciliation interval by 2 then adding that to
	// `startsAt` (the next successful reconciliation would make sure
	// the alert is cleared after the timeout). Due to
	// event.InvolvedObject only containing the object reference (namely
	// the GVKNN) best we can do is leave it unset up to Alertmanager's
	// default `resolve_timeout`.
	//
	// https://prometheus.io/docs/alerting/0.27/configuration/#file-layout-and-global-settings
	startsAt := AlertManagerTime(event.Timestamp.Time)

	payload := []AlertManagerAlert{
		{
			Labels:      labels,
			Annotations: annotations,
			Status:      "firing",

			StartsAt: startsAt,
		},
	}

	var opts []postOption
	if s.ProxyURL != "" {
		opts = append(opts, withProxy(s.ProxyURL))
	}
	if s.TLSConfig != nil {
		opts = append(opts, withTLSConfig(s.TLSConfig))
	}
	if s.Token != "" {
		opts = append(opts, withRequestModifier(func(request *retryablehttp.Request) {
			request.Header.Add("Authorization", "Bearer "+s.Token)
		}))
	}
	if s.Username != "" && s.Password != "" {
		opts = append(opts, withRequestModifier(func(request *retryablehttp.Request) {
			request.SetBasicAuth(s.Username, s.Password)
		}))
	}

	if err := postMessage(ctx, s.URL, payload, opts...); err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}

	return nil
}
