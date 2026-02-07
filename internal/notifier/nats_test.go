package notifier

import (
	"context"
	"errors"
	"fmt"
	"testing"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/nats-io/nats.go"
	. "github.com/onsi/gomega"
)

func TestNewNATS(t *testing.T) {
	tests := []struct {
		name             string
		subject          string
		server           string
		username         string
		password         string
		secretData       map[string][]byte
		expectedErr      error
		expectedSubject  string
		expectedUsername string
		expectedPassword string
		expectedCreds    bool
		expectedNkey     bool
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
		{
			name:            "credentials file data is stored",
			subject:         "test",
			server:          "nats",
			secretData:      map[string][]byte{"creds": []byte("credentials-content")},
			expectedSubject: "test",
			expectedCreds:   true,
		},
		{
			name:            "nkey seed data is stored",
			subject:         "test",
			server:          "nats",
			secretData:      map[string][]byte{"nkey": []byte("SUAGMJH5XLGZKQQWAWKRZJIGMOU4HPFUYLXJMXOO5NLFEO2OOQJ5LPRDPM")},
			expectedSubject: "test",
			expectedNkey:    true,
		},
		// Priority tests: creds > nkey > username/password
		{
			name:     "creds takes priority over nkey and username/password",
			subject:  "test",
			server:   "nats",
			username: "user",
			password: "pass",
			secretData: map[string][]byte{
				"creds": []byte("credentials-content"),
				"nkey":  []byte("SUAGMJH5XLGZKQQWAWKRZJIGMOU4HPFUYLXJMXOO5NLFEO2OOQJ5LPRDPM"),
			},
			expectedSubject: "test",
			expectedCreds:   true,
		},
		{
			name:            "nkey takes priority over username/password",
			subject:         "test",
			server:          "nats",
			username:        "user",
			password:        "pass",
			secretData:      map[string][]byte{"nkey": []byte("SUAGMJH5XLGZKQQWAWKRZJIGMOU4HPFUYLXJMXOO5NLFEO2OOQJ5LPRDPM")},
			expectedSubject: "test",
			expectedNkey:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewNATS(tt.server, tt.subject, tt.username, tt.password, tt.secretData)
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

				// Verify authFn returns valid option and correct auth type
				if client.authFn != nil {
					opt, cleanup, err := client.authFn()
					g.Expect(err).To(BeNil(), "authFn should not return error")
					g.Expect(opt).NotTo(BeNil(), "authFn should return a valid option")
					if cleanup != nil {
						defer cleanup()
					}

					// Apply option to verify which auth method was selected
					var opts nats.Options
					g.Expect(opt(&opts)).To(Succeed())

					if tt.expectedCreds {
						g.Expect(opts.UserJWT).NotTo(BeNil(), "creds auth should set UserJWT nats.Options field")
					} else if tt.expectedNkey {
						g.Expect(opts.Nkey).NotTo(BeEmpty(), "nkey auth should set Nkey nats.Options field")
						g.Expect(opts.SignatureCB).NotTo(BeNil(), "nkey auth should set SignatureCB nats.Options field")
					} else if tt.expectedUsername != "" {
						g.Expect(opts.User).To(Equal(tt.expectedUsername), "username/password auth should set User nats.Options field")
						g.Expect(opts.Password).To(Equal(tt.expectedPassword), "username/password auth should set Password nats.Options field")
					}
				} else {
					// Verify authFn is configured based on authentication type
					if tt.username != "" || tt.password != "" {
						g.Expect(client.authFn).NotTo(BeNil(), "authFn should be set for username/password auth")
					} else if tt.secretData != nil && tt.secretData["creds"] != nil {
						g.Expect(client.authFn).NotTo(BeNil(), "authFn should be set for credentials file auth")
					} else if tt.secretData != nil && tt.secretData["nkey"] != nil {
						g.Expect(client.authFn).NotTo(BeNil(), "authFn should be set for nkey auth")
					} else {
						// When no auth is provided, authFn should be nil
						g.Expect(client.authFn).To(BeNil(), "authFn should be nil when no auth is provided")
						g.Expect(tt.expectedCreds).To(BeFalse())
						g.Expect(tt.expectedNkey).To(BeFalse())
						g.Expect(tt.expectedUsername).To(BeEmpty())
						g.Expect(tt.expectedPassword).To(BeEmpty())
					}
				}
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
