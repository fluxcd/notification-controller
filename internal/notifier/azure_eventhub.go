/*
Copyright 2021 The Flux authors
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
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-amqp-common-go/v3/auth"
	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"github.com/fluxcd/pkg/runtime/events"
)

// AzureEventHub holds the eventhub client
type AzureEventHub struct {
	Hub *eventhub.Hub
}

// NewAzureEventHub creates a eventhub client
func NewAzureEventHub(endpointURL string) (*AzureEventHub, error) {
	var hub *eventhub.Hub
	var err error

	if strings.ToLower(endpointURL[:8]) != "endpoint" {
		hub, err = newJWTHub(endpointURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create a eventhub using JWT %v", err)
		}
	} else {
		hub, err = newSASHub(endpointURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create a eventhub using SAS %v", err)
		}
	}

	return &AzureEventHub{
		Hub: hub,
	}, nil
}

// Post all notification-controller messages to EventHub
func (e *AzureEventHub) Post(event events.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("Unable to marshall event: %w", err)
	}

	err = e.Hub.Send(ctx, eventhub.NewEvent(eventBytes))
	if err != nil {
		return fmt.Errorf("Failed to send msg: %w", err)
	}

	err = e.Hub.Close(ctx)
	if err != nil {
		return fmt.Errorf("Unable to close connection: %w", err)
	}
	return nil
}

// PureJWT just contains the jwt
type PureJWT struct {
	jwt string
}

// NewJWTProvider create a pureJWT method
func NewJWTProvider(jwt string) *PureJWT {
	return &PureJWT{
		jwt: jwt,
	}
}

// GetToken uses a JWT token, we assume that we will get new tokens when needed, thus no Expiry defined
func (j *PureJWT) GetToken(uri string) (*auth.Token, error) {
	return &auth.Token{
		TokenType: auth.CBSTokenTypeJWT,
		Token:     j.jwt,
		Expiry:    "",
	}, nil
}

// newJWTHub used when address is a JWT token
func newJWTHub(address string) (*eventhub.Hub, error) {
	provider := NewJWTProvider(address)

	hub, err := eventhub.NewHub("fluxv2", "fluxv2", provider)
	if err != nil {
		return nil, err
	}
	return hub, nil
}

// newSASHub used when address is a SAS ConnectionString
func newSASHub(address string) (*eventhub.Hub, error) {
	hub, err := eventhub.NewHubFromConnectionString(address)
	if err != nil {
		return nil, err
	}

	return hub, nil
}
