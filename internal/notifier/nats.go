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
	"encoding/json"
	"errors"
	"fmt"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/nats-io/nats.go"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type (
	// NATS holds a NATS client and target subject.
	NATS struct {
		subject string
		client  interface {
			publish(ctx context.Context, subject string, eventPayload []byte) (err error)
		}
	}

	natsClient struct {
		server   string
		username string
		password string
	}
)

func NewNATS(server string, subject string, username string, password string) (*NATS, error) {
	if server == "" {
		return nil, errors.New("NATS server (address) cannot be empty")
	}
	if subject == "" {
		return nil, errors.New("NATS subject (channel) cannot be empty")
	}
	return &NATS{
		subject: subject,
		client: &natsClient{
			server:   server,
			username: username,
			password: password,
		},
	}, nil
}

// Post posts Flux events to a NATS subject.
func (n *NATS) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	eventPayload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error json-marshaling event: %w", err)
	}

	err = n.client.publish(ctx, n.subject, eventPayload)
	if err != nil {
		return fmt.Errorf("error publishing event to subject %s: %w", n.subject, err)
	}

	// debug log
	log.FromContext(ctx).V(1).Info("Event published to NATS subject", "subject", n.subject)

	return nil
}

func (n *natsClient) publish(ctx context.Context, subject string, eventPayload []byte) (err error) {
	opts := []nats.Option{nats.Name("NATS Provider Publisher")}
	if n.username != "" && n.password != "" {
		opts = append(opts, nats.UserInfo(n.username, n.password))
	}

	nc, err := nats.Connect(n.server, opts...)
	if err != nil {
		return fmt.Errorf("error connecting to server: %w", err)
	}
	defer nc.Close()

	nc.Publish(subject, eventPayload)
	nc.Flush()
	if err = nc.LastError(); err != nil {
		return fmt.Errorf("error publishing message to server: %w", err)
	}

	return err
}
