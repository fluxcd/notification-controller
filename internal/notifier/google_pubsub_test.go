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
	"testing"

	. "github.com/onsi/gomega"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestNewGooglePubSub(t *testing.T) {
	tests := []struct {
		name              string
		projectID         string
		topicID           string
		jsonCreds         string
		attrs             map[string]string
		expectedErr       error
		expectedTopicName string
		expectedJSONCreds []byte
		expectedAttrs     map[string]string
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
		{
			name:              "topic name is stored properly",
			projectID:         "project-id",
			topicID:           "topic-id",
			expectedTopicName: "projects/project-id/topics/topic-id",
		},
		{
			name:              "json creds are stored properly",
			projectID:         "project-id",
			topicID:           "topic-id",
			expectedTopicName: "projects/project-id/topics/topic-id",
			jsonCreds:         "json credentials",
			expectedJSONCreds: []byte("json credentials"),
		},
		{
			name:              "non-empty attributes are stored properly",
			projectID:         "project-id",
			topicID:           "topic-id",
			expectedTopicName: "projects/project-id/topics/topic-id",
			attrs:             map[string]string{"foo": "bar"},
			expectedAttrs:     map[string]string{"foo": "bar"},
		},
		{
			name:              "empty attributes are stored properly",
			projectID:         "project-id",
			topicID:           "topic-id",
			expectedTopicName: "projects/project-id/topics/topic-id",
			attrs:             map[string]string{},
			expectedAttrs:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewGooglePubSub(tt.projectID, tt.topicID, tt.jsonCreds, tt.attrs)
			if tt.expectedErr != nil {
				g.Expect(err).To(Equal(tt.expectedErr))
				g.Expect(provider).To(BeNil())
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(provider).NotTo(BeNil())

				g.Expect(provider.topicID).To(Equal(tt.topicID))
				g.Expect(provider.attrs).To(Equal(tt.expectedAttrs))
				g.Expect(provider.topicName).To(Equal(tt.expectedTopicName))

				g.Expect(provider.client).NotTo(BeNil())
				client := provider.client.(*googlePubSubClient)
				g.Expect(client).NotTo(BeNil())

				g.Expect(client.projectID).To(Equal(tt.projectID))
				g.Expect(client.jsonCreds).To(Equal(tt.expectedJSONCreds))
			}
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

func (tt *googlePubSubPostTestCase) publish(ctx context.Context, topicID string, eventPayload []byte, attrs map[string]string) (serverID string, err error) {
	tt.g.THelper()
	tt.publishExecuted = true
	tt.g.Expect(topicID).To(Equal(tt.topicID))
	tt.g.Expect(string(eventPayload)).To(Equal(tt.expectedEventPayload))
	tt.g.Expect(attrs).To(Equal(tt.attrs))
	// serverID is only used in a debug log for now, there's no way to assert it
	return "", tt.publishErr
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
			name: "commit status updates are dropped",
			event: eventv1.Event{
				Metadata: map[string]string{"commit_status": "update"},
			},
			publishShouldExecute: false,
		},
		{
			name:                 "publish error is wrapped and relayed",
			expectedEventPayload: `{"involvedObject":{},"severity":"","timestamp":null,"message":"","reason":"","reportingController":""}`,
			topicName:            "projects/projectID/topics/topicID",
			publishErr:           errors.New("publish error"),
			expectedErr:          fmt.Errorf("error publishing event to topic projects/projectID/topics/topicID: %w", errors.New("publish error")),
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
				client:    tt,
				topicID:   tt.topicID,
				attrs:     tt.attrs,
				topicName: tt.topicName,
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
