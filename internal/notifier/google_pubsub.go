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

	"cloud.google.com/go/pubsub"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type (
	// GooglePubSub holds a Google Pub/Sub client and target topic.
	GooglePubSub struct {
		client interface {
			publish(ctx context.Context, eventPayload []byte) error
		}
	}

	googlePubSubClient struct {
		client    *pubsub.Client
		projectID string
		topicID   string
		headers   map[string]string
	}
)

// ensure *GooglePubSub implements Interface.
var _ Interface = &GooglePubSub{}

// NewGooglePubSub creates a Google Pub/Sub client tied to a specific
// project and topic using the provided client options.
func NewGooglePubSub(opts *notifierOptions) (*GooglePubSub, error) {
	if opts.URL == "" {
		return nil, errors.New("GCP project ID cannot be empty")
	}
	if opts.Channel == "" {
		return nil, errors.New("GCP Pub/Sub topic ID cannot be empty")
	}

	clientOpts, err := buildGCPClientOptions(opts.Context, *opts)
	if err != nil {
		return nil, fmt.Errorf("failed to build GCP client options: %w", err)
	}
	client, err := pubsub.NewClient(opts.Context, opts.URL, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Pub/Sub client: %w", err)
	}

	pubsubClient := &googlePubSubClient{
		client:    client,
		projectID: opts.URL,
		topicID:   opts.Channel,
		headers:   opts.Headers,
	}

	return &GooglePubSub{
		client: pubsubClient,
	}, nil
}

// Post posts Flux events to a Google Pub/Sub topic.
func (g *GooglePubSub) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	eventPayload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error json-marshaling event: %w", err)
	}

	return g.client.publish(ctx, eventPayload)
}

func (g *googlePubSubClient) publish(ctx context.Context, eventPayload []byte) error {
	// Publish the event to the topic.
	attrs := g.headers
	if len(attrs) == 0 {
		attrs = nil
	}
	topic := fmt.Sprintf("projects/%s/topics/%s", g.projectID, g.topicID)
	serverID, err := g.client.
		Topic(g.topicID).
		Publish(ctx, &pubsub.Message{
			Data:       eventPayload,
			Attributes: attrs,
		}).
		Get(ctx)
	if err != nil {
		return fmt.Errorf("error publishing to GCP Pub/Sub topic %s: %w", topic, err)
	}

	// Emit debug log.
	log.FromContext(ctx).V(1).Info("Event published to GCP Pub/Sub topic",
		"topic", topic,
		"server message id", serverID)

	return nil
}
