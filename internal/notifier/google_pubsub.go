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
	"google.golang.org/api/option"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type (
	// GooglePubSub holds a Google Pub/Sub client and target topic.
	GooglePubSub struct {
		topicID   string
		attrs     map[string]string
		topicName string

		client interface {
			publish(ctx context.Context, topicID string, eventPayload []byte, attrs map[string]string) (serverID string, err error)
		}
	}

	googlePubSubClient struct {
		projectID string
		opts      []option.ClientOption
	}
)

// ensure *GooglePubSub implements Interface.
var _ Interface = &GooglePubSub{}

// NewGooglePubSub creates a Google Pub/Sub client tied to a specific
// project and topic using the provided client options.
func NewGooglePubSub(ctx context.Context, projectID, topicID string, attrs map[string]string,
	clientOpts []option.ClientOption) (*GooglePubSub, error) {
	if projectID == "" {
		return nil, errors.New("GCP project ID cannot be empty")
	}
	if topicID == "" {
		return nil, errors.New("GCP Pub/Sub topic ID cannot be empty")
	}
	if len(attrs) == 0 {
		attrs = nil
	}

	client := &googlePubSubClient{
		projectID: projectID,
		opts:      clientOpts,
	}

	return &GooglePubSub{
		topicID:   topicID,
		attrs:     attrs,
		topicName: fmt.Sprintf("projects/%s/topics/%s", projectID, topicID),
		client:    client,
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

	serverID, err := g.client.publish(ctx, g.topicID, eventPayload, g.attrs)
	if err != nil {
		return fmt.Errorf("error publishing event to topic %s: %w", g.topicName, err)
	}

	// debug log
	log.FromContext(ctx).V(1).Info("Event published to GCP Pub/Sub topic",
		"topic", g.topicName,
		"server message id", serverID)

	return nil
}

func (g *googlePubSubClient) publish(ctx context.Context, topicID string, eventPayload []byte, attrs map[string]string) (serverID string, err error) {
	var client *pubsub.Client
	client, err = pubsub.NewClient(ctx, g.projectID, g.opts...)
	if err != nil {
		return
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			if err != nil {
				err = kerrors.NewAggregate([]error{err, closeErr})
			} else {
				err = closeErr
			}
		}
	}()
	serverID, err = client.
		Topic(topicID).
		Publish(ctx, &pubsub.Message{
			Data:       eventPayload,
			Attributes: attrs,
		}).
		Get(ctx)
	return
}
