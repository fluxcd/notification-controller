/*
Copyright 2025 The Flux authors

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
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	eventhub "github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/cache"
	fluxbroker "github.com/fluxcd/pkg/spiffe/broker"
)

// AzureEventHub holds the eventhub client
type AzureEventHub struct {
	ProducerClient *eventhub.ProducerClient
}

// NewAzureEventHub creates a eventhub client
func NewAzureEventHub(ctx context.Context, endpointURL, token, eventHubNamespace, proxy,
	serviceAccountName, providerName, providerNamespace string, tokenClient client.Client,
	tokenCache *cache.TokenCache, spiffeBroker fluxbroker.Interface, defaultServiceAccount string) (*AzureEventHub, error) {
	var producerClient *eventhub.ProducerClient
	var err error

	if err := validateAuthOptions(endpointURL, token, serviceAccountName); err != nil {
		return nil, fmt.Errorf("invalid authentication options: %v", err)
	}

	if isSASAuth(endpointURL) {
		producerClient, err = newSASHub(endpointURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create a eventhub using SAS: %w", err)
		}
	} else {
		accessToken, err := newManagedIdentityToken(ctx, proxy, serviceAccountName,
			defaultServiceAccount, providerName,
			providerNamespace, azure.ScopeEventHubs, tokenClient, tokenCache, spiffeBroker)
		if err != nil {
			return nil, fmt.Errorf("failed to create a eventhub using managed identity: %w", err)
		}
		producerClient, err = newJWTHub(endpointURL, accessToken, eventHubNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to create a eventhub using authentication token: %w", err)
		}
	}

	return &AzureEventHub{
		ProducerClient: producerClient,
	}, nil
}

// Post all notification-controller messages to EventHub
func (e *AzureEventHub) Post(ctx context.Context, event eventv1.Event) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("unable to marshall event: %w", err)
	}

	eventBatch, err := e.ProducerClient.NewEventDataBatch(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create event data batch: %w", err)
	}

	err = eventBatch.AddEventData(&eventhub.EventData{Body: eventBytes}, nil)
	if err != nil {
		return fmt.Errorf("failed to add event data to batch: %w", err)
	}

	err = e.ProducerClient.SendEventDataBatch(ctx, eventBatch, nil)
	if err != nil {
		return fmt.Errorf("failed to send msg: %w", err)
	}

	err = e.ProducerClient.Close(ctx)
	if err != nil {
		return fmt.Errorf("unable to close connection: %w", err)
	}
	return nil
}

// PureJWT just contains the jwt
type PureJWT struct {
	jwt string
}

// NewJWTProvider create a pureJWT method
func NewJWTProvider(jwt string) azcore.TokenCredential {
	return &PureJWT{
		jwt: jwt,
	}
}

// GetToken uses a JWT token, we assume that we will get new tokens when needed, thus no Expiry defined
func (j *PureJWT) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token: j.jwt,
	}, nil
}

// newJWTHub creates a producer client authenticated with the given access token.
func newJWTHub(eventhubName, token, eventHubNamespace string) (*eventhub.ProducerClient, error) {
	provider := NewJWTProvider(token)
	fullyQualifiedNamespace := ensureFullyQualifiedNamespace(eventHubNamespace)
	hub, err := eventhub.NewProducerClient(fullyQualifiedNamespace, eventhubName, provider, nil)
	if err != nil {
		return nil, err
	}
	return hub, nil
}

// newSASHub used when address is a SAS ConnectionString
func newSASHub(address string) (*eventhub.ProducerClient, error) {
	producerClient, err := eventhub.NewProducerClientFromConnectionString(address, "", nil)
	if err != nil {
		return nil, err
	}

	return producerClient, nil
}

// validateAuthOptions checks if the authentication options are valid
func validateAuthOptions(endpointURL, token, serviceAccountName string) error {
	if endpointURL == "" {
		return fmt.Errorf("endpoint URL cannot be empty")
	}

	if token != "" {
		return fmt.Errorf("static JWT authentication with the 'token' key is no longer supported, " +
			"use workload identity or SAS authentication instead")
	}

	if isSASAuth(endpointURL) && serviceAccountName != "" {
		return fmt.Errorf("serviceAccountName and SAS authentication cannot be set at the same time")
	}

	return nil
}

// isSASAuth checks if the endpoint URL contains SAS authentication parameters
func isSASAuth(endpointURL string) bool {
	return strings.Contains(endpointURL, "SharedAccessKey")
}

// getEventHubSuffixFromAuthorityHost maps AZURE_AUTHORITY_HOST to the correct suffix
func getEventHubSuffixFromAuthorityHost() string {
	authorityHost := os.Getenv("AZURE_AUTHORITY_HOST")
	switch {
	case strings.Contains(authorityHost, "chinacloudapi.cn"):
		return ".servicebus.chinacloudapi.cn"
	case strings.Contains(authorityHost, "microsoftonline.us"):
		return ".servicebus.usgovcloudapi.net"
	default:
		return ".servicebus.windows.net"
	}
}

// ensureFullyQualifiedNamespace appends suffix if not already present
func ensureFullyQualifiedNamespace(namespace string) string {
	if strings.Contains(namespace, ".servicebus.") {
		return namespace // already fully qualified
	}
	return namespace + getEventHubSuffixFromAuthorityHost()
}
