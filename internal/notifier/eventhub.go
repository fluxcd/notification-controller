/*
Copyright 2020 The Flux authors
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
	"time"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"github.com/fluxcd/pkg/runtime/events"
)

type EventHub struct {
	Hub *eventhub.Hub
}

func NewEventHub(endpointURL string) (*EventHub, error) {
	hub, err := eventhub.NewHubFromConnectionString(endpointURL)
	if err != nil {
		return nil, err
	}
	return &EventHub{
		Hub: hub,
	}, nil
}

// Post EventHub msg
func (e *EventHub) Post(event events.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("Unable to marshall event: %w", err)
	}

	//err := e.Hub.Send(ctx, eventhub.NewEventFromString("From, String"))
	err = e.Hub.Send(ctx, eventhub.NewEvent(eventBytes))
	if err != nil {
		return fmt.Errorf("Failed to send msg: %w", err)
	}

	err = e.Hub.Close(context.Background())
	if err != nil {
		return fmt.Errorf("Unable to close connection: %w", err)
	}
	return nil
}
