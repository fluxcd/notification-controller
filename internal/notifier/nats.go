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
	"github.com/nats-io/nkeys"
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
		server    string
		authFn    func() (nats.Option, func(), error)
		username  string
		password  string
		credsData []byte
		nkeySeed  []byte
	}
)

// NewNATS creates a new NATS notifier with support for multiple authentication methods.
//
// Authentication methods (in priority order):
//  1. User Credentials (JWT + NKey): Pass the .creds file content via credsData parameter
//  2. NKey: Pass the NKey seed via nkeySeed parameter
//  3. Username/Password: Pass via username and password parameters
//
// Parameters:
//   - server: NATS server URL (e.g., "nats://localhost:4222")
//   - subject: NATS subject to publish events to
//   - username: Username for basic authentication (optional)
//   - password: Password for basic authentication (optional)
//   - credsData: User credentials file content (JWT + NKey) for NATS 2.0+ authentication (optional)
//   - nkeySeed: NKey seed for NKey-based authentication (optional)
//
// Returns an error if server or subject is empty.
func NewNATS(server string, subject string, username string, password string, secretData map[string][]byte) (*NATS, error) {
	if server == "" {
		return nil, errors.New("NATS server (address) cannot be empty")
	}
	if subject == "" {
		return nil, errors.New("NATS subject (channel) cannot be empty")
	}

	client := &natsClient{server: server}

	// Extract credentials from secret data
	// Keys: "creds" for user credentials file, "nkey" for nkey seed
	var credsData, nkeySeed []byte
	if secretData != nil {
		credsData = secretData["creds"]
		nkeySeed = secretData["nkey"]
	}

	// Set up authentication function based on provided credentials
	// Authentication priority: user credentials (JWT), nkey, username/password
	if len(credsData) > 0 {
		client.authFn = func() (nats.Option, func(), error) {
			return nats.UserCredentialBytes(credsData), nil, nil
		}
	} else if len(nkeySeed) > 0 {
		client.authFn = func() (nats.Option, func(), error) {
			kp, err := nkeys.FromSeed(nkeySeed)
			if err != nil {
				return nil, nil, fmt.Errorf("error parsing nkey seed: %w", err)
			}
			pubKey, err := kp.PublicKey()
			if err != nil {
				kp.Wipe()
				return nil, nil, fmt.Errorf("error getting public key from nkey: %w", err)
			}
			// Create signature callback
			sigCB := func(nonce []byte) ([]byte, error) {
				return kp.Sign(nonce)
			}
			return nats.Nkey(pubKey, sigCB), kp.Wipe, nil
		}
	} else if username != "" && password != "" {
		client.authFn = func() (nats.Option, func(), error) {
			return nats.UserInfo(username, password), nil, nil
		}
	}

	return &NATS{
		subject: subject,
		client:  client,
	}, nil
}

// Post posts Flux events to a NATS subject.
func (n *NATS) Post(ctx context.Context, event eventv1.Event) error {
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

	// Apply authentication if configured
	if n.authFn != nil {
		authOpt, cleanup, err := n.authFn()
		if err != nil {
			return err
		}
		if cleanup != nil {
			defer cleanup()
		}
		opts = append(opts, authOpt)
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
