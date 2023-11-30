package notifier

import (
	"context"
	"errors"
	"fmt"
	"testing"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	. "github.com/onsi/gomega"
)

func TestNewNATS(t *testing.T) {
	tests := []struct {
		name             string
		subject          string
		server           string
		username         string
		password         string
		expectedErr      error
		expectedSubject  string
		expectedUsername string
		expectedPassword string
	}{
		{
			name:        "empty subject is not allowed",
			subject:     "",
			server:      "nats",
			expectedErr: errors.New("NATS subject (channel) cannot be empty"),
		},
		{
			name:        "empty server is not allowed",
			subject:     "test",
			server:      "",
			expectedErr: errors.New("NATS server (address) cannot be empty"),
		},
		{
			name:             "empty creds are stored properly",
			subject:          "test",
			server:           "nats",
			username:         "",
			password:         "",
			expectedSubject:  "test",
			expectedUsername: "",
			expectedPassword: "",
		},
		{
			name:             "non-empty creds are stored properly",
			subject:          "test",
			server:           "nats",
			username:         "user",
			password:         "pass",
			expectedSubject:  "test",
			expectedUsername: "user",
			expectedPassword: "pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewNATS(tt.server, tt.subject, tt.username, tt.password)
			if tt.expectedErr != nil {
				g.Expect(err).To(Equal(tt.expectedErr))
				g.Expect(provider).To(BeNil())
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(provider).NotTo(BeNil())

				g.Expect(provider.subject).To(Equal(tt.expectedSubject))

				g.Expect(provider.client).NotTo(BeNil())
				client := provider.client.(*natsClient)
				g.Expect(client).NotTo(BeNil())

				g.Expect(client.server).To(Equal(tt.server))
				g.Expect(client.username).To(Equal(tt.expectedUsername))
				g.Expect(client.password).To(Equal(tt.expectedPassword))
			}
		})
	}
}

type natsPostTestCase struct {
	name                 string
	subject              string
	event                eventv1.Event
	expectedEventPayload string
	publishErr           error
	expectedErr          error
	publishShouldExecute bool
	publishExecuted      bool

	g *WithT
}

func (tt *natsPostTestCase) publish(ctx context.Context, subject string, eventPayload []byte) (err error) {
	tt.g.THelper()
	tt.publishExecuted = true
	tt.g.Expect(subject).To(Equal(tt.subject))
	tt.g.Expect(string(eventPayload)).To(Equal(tt.expectedEventPayload))
	return tt.publishErr
}

func TestNATSPost(t *testing.T) {
	tests := []*natsPostTestCase{
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
			subject:              "test",
			expectedEventPayload: `{"involvedObject":{},"severity":"","timestamp":null,"message":"","reason":"","reportingController":""}`,
			publishErr:           errors.New("publish error"),
			expectedErr:          fmt.Errorf("error publishing event to subject test: %w", errors.New("publish error")),
			publishShouldExecute: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tt.g = g

			topic := &NATS{
				client:  tt,
				subject: tt.subject,
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
