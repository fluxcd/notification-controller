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
	"crypto/x509"
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxcd/pkg/runtime/events"
)

type Alertmanager struct {
	URL      string
	ProxyURL string
	CertPool *x509.CertPool
}

type AlertManagerAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func NewAlertmanager(hookURL string, proxyURL string, certPool *x509.CertPool) (*Alertmanager, error) {
	_, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Alertmanager URL %s", hookURL)
	}

	return &Alertmanager{
		URL:      hookURL,
		ProxyURL: proxyURL,
		CertPool: certPool,
	}, nil
}

func (s *Alertmanager) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	annotations := make(map[string]string)
	annotations["message"] = event.Message

	_, ok := event.Metadata["summary"]
	if ok {
		annotations["summary"] = event.Metadata["summary"]
		delete(event.Metadata, "summary")
	}

	labels := event.Metadata
	labels["alertname"] = "Flux" + event.InvolvedObject.Kind + strings.Title(event.Reason)
	labels["severity"] = event.Severity
	labels["reason"] = event.Reason

	labels["kind"] = event.InvolvedObject.Kind
	labels["name"] = event.InvolvedObject.Name
	labels["namespace"] = event.InvolvedObject.Namespace
	labels["reportingcontroller"] = event.ReportingController

	payload := []AlertManagerAlert{
		{
			Labels:      labels,
			Annotations: annotations,
			Status:      "firing",
		},
	}

	err := postMessage(s.URL, s.ProxyURL, s.CertPool, payload)

	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
