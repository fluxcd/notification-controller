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
	"testing"

	. "github.com/onsi/gomega"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestNewGooglePubSub(t *testing.T) {
	tests := []struct {
		name        string
		projectID   string
		topicID     string
		expectedErr error
	}{
		{
			name:        "empty project ID is not allowed",
			projectID:   "",
			expectedErr: errors.New("GCP project ID cannot be empty"),
		},
		{
			name:        "empty topic ID is not allowed",
			projectID:   "project-id",
			topicID:     "",
			expectedErr: errors.New("GCP Pub/Sub topic ID cannot be empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			provider, err := NewGooglePubSub(&notifierOptions{
				URL:     tt.projectID,
				Channel: tt.topicID,
			})

			g.Expect(err).To(Equal(tt.expectedErr))
			g.Expect(provider).To(BeNil())
		})
	}
}

type googlePubSubPostTestCase struct {
	name                 string
	topicID              string
	attrs                map[string]string
	topicName            string
	event                eventv1.Event
	expectedEventPayload string
	publishErr           error
	expectedErr          error
	publishShouldExecute bool
	publishExecuted      bool

	g *WithT
}

func (tt *googlePubSubPostTestCase) publish(ctx context.Context, eventPayload []byte) error {
	tt.g.THelper()
	tt.publishExecuted = true
	tt.g.Expect(string(eventPayload)).To(Equal(tt.expectedEventPayload))
	return tt.publishErr
}

func TestGooglePubSubPost(t *testing.T) {
	tests := []*googlePubSubPostTestCase{
		{
			name: "events are properly marshaled",
			event: eventv1.Event{
				Metadata: map[string]string{"foo": "bar"},
			},
			expectedEventPayload: `{"involvedObject":{},"severity":"","timestamp":null,"message":"","reason":"","metadata":{"foo":"bar"},"reportingController":""}`,
			publishShouldExecute: true,
		},
		{
			name:                 "publish error is relayed",
			expectedEventPayload: `{"involvedObject":{},"severity":"","timestamp":null,"message":"","reason":"","reportingController":""}`,
			topicName:            "projects/projectID/topics/topicID",
			publishErr:           errors.New("publish error"),
			expectedErr:          errors.New("publish error"),
			publishShouldExecute: true,
		},
		{
			name:                 "topic and attributes are relayed to the internal client",
			topicID:              "topicID",
			attrs:                map[string]string{"foo": "bar"},
			expectedEventPayload: `{"involvedObject":{},"severity":"","timestamp":null,"message":"","reason":"","reportingController":""}`,
			publishShouldExecute: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tt.g = g

			topic := &GooglePubSub{
				client: tt,
			}

			err := topic.Post(context.Background(), tt.event)
			if tt.expectedErr == nil {
				g.Expect(err).To(BeNil())
			} else {
				g.Expect(err).To(Equal(tt.expectedErr))
			}
			g.Expect(tt.publishExecuted).To(Equal(tt.publishShouldExecute))
		})
	}
}
