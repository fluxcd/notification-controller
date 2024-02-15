/*
Copyright 2023 The Flux authors

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
	"errors"
	"fmt"
	"net/url"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/hashicorp/go-retryablehttp"
)

const (
	NtfyTagInfo  = "information_source"
	NtfyTagError = "rotating_light"
)

type Ntfy struct {
	ServerURL string
	Topic     string
	Token     string
	Username  string
	Password  string
}

type NtfyMessage struct {
	Topic   string   `json:"topic"`
	Message string   `json:"message"`
	Title   string   `json:"title"`
	Tags    []string `json:"tags,omitempty"`
}

func NewNtfy(serverURL string, topic string, token string, username string, password string) (*Ntfy, error) {
	_, err := url.ParseRequestURI(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Ntfy server URL %s: '%w'", serverURL, err)
	}

	if topic == "" {
		return nil, errors.New("ntfy topic cannot be empty")
	}

	return &Ntfy{
		ServerURL: serverURL,
		Topic:     topic,
		Token:     token,
		Username:  username,
		Password:  password,
	}, nil
}

func (n *Ntfy) Post(ctx context.Context, event eventv1.Event) error {

	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	tags := make([]string, 0)

	switch event.Severity {
	case eventv1.EventSeverityInfo:
		tags = append(tags, NtfyTagInfo)
	case eventv1.EventSeverityError:
		tags = append(tags, NtfyTagError)
	}

	payload := NtfyMessage{
		Topic:   n.Topic,
		Title:   fmt.Sprintf("FluxCD: %s", event.ReportingController),
		Message: n.buildMessageFromEvent(event),
		Tags:    tags,
	}

	err := postMessage(ctx, n.ServerURL, "", nil, payload, func(req *retryablehttp.Request) {
		n.addAuthorizationHeader(req)
	})

	return err
}

func (n *Ntfy) addAuthorizationHeader(req *retryablehttp.Request) {
	if n.Username != "" && n.Password != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", basicAuth(n.Username, n.Password)))
	} else if n.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", n.Token))
	}
}

func (n *Ntfy) buildMessageFromEvent(event eventv1.Event) string {
	var messageBuilder strings.Builder

	messageBuilder.WriteString(fmt.Sprintf("%s\n\n", event.Message))
	messageBuilder.WriteString(fmt.Sprintf("Object: %s/%s.%s\n", event.InvolvedObject.Namespace, event.InvolvedObject.Name, event.InvolvedObject.Kind))

	if event.Metadata != nil {
		messageBuilder.WriteString("\nMetadata:\n")
		for key, val := range event.Metadata {
			messageBuilder.WriteString(fmt.Sprintf("%s: %s\n", key, val))
		}
	}

	return messageBuilder.String()
}
