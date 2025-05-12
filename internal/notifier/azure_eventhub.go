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
	"net/url"
	"strings"

	"github.com/Azure/azure-amqp-common-go/v4/auth"
	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	pkgauth "github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/cache"
	pkgcache "github.com/fluxcd/pkg/cache"

	"github.com/fluxcd/notification-controller/api/v1beta3"
)

// AzureEventHub holds the eventhub client
type AzureEventHub struct {
	Hub *eventhub.Hub
}

// NewAzureEventHub creates a eventhub client
func NewAzureEventHub(ctx context.Context, endpointURL, token, eventHubNamespace, proxy, serviceAccountName, providerName, providerNamespace string, tokenClient client.Client, tokenCache *pkgcache.TokenCache) (*AzureEventHub, error) {
	var hub *eventhub.Hub
	var err error

	if err := validateAuthOptions(endpointURL, token, serviceAccountName); err != nil {
		return nil, fmt.Errorf("invalid authentication options: %v", err)
	}

	if isSASAuth(endpointURL) {
		hub, err = newSASHub(endpointURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create a eventhub using SAS %v", err)
		}
	} else {
		// if token doesn't exist, try to create a new token using managed identity
		if token == "" {
			token, err = newManagedIdentityToken(ctx, proxy, serviceAccountName, providerName, providerNamespace, tokenClient, tokenCache)
			if err != nil {
				return nil, fmt.Errorf("failed to create a eventhub using managed identity %v", err)
			}
		}
		hub, err = newJWTHub(endpointURL, token, eventHubNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to create a eventhub using authentication token %v", err)
		}
	}

	return &AzureEventHub{
		Hub: hub,
	}, nil
}

// Post all notification-controller messages to EventHub
func (e *AzureEventHub) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("unable to marshall event: %w", err)
	}

	err = e.Hub.Send(ctx, eventhub.NewEvent(eventBytes))
	if err != nil {
		return fmt.Errorf("failed to send msg: %w", err)
	}

	err = e.Hub.Close(ctx)
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
func newJWTHub(eventhubName, token, eventHubNamespace string) (*eventhub.Hub, error) {
	provider := NewJWTProvider(token)

	hub, err := eventhub.NewHub(eventHubNamespace, eventhubName, provider)
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

// newManagedIdentityToken is used to attempt credential-free authentication.
func newManagedIdentityToken(ctx context.Context, proxy, serviceAccountName, providerName, providerNamespace string, tokenClient client.Client, tokenCache *pkgcache.TokenCache) (string, error) {
	opts := []pkgauth.Option{pkgauth.WithScopes(azure.ScopeEventHubs)}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return "", fmt.Errorf("error parsing proxy URL : %w", err)
		}
		opts = append(opts, pkgauth.WithProxyURL(*proxyURL))
	}

	if serviceAccountName != "" {
		serviceAccount := types.NamespacedName{
			Name:      serviceAccountName,
			Namespace: providerNamespace,
		}
		opts = append(opts, pkgauth.WithServiceAccount(serviceAccount, tokenClient))
	}

	if tokenCache != nil {
		involvedObject := cache.InvolvedObject{
			Kind:      v1beta3.ProviderKind,
			Name:      providerName,
			Namespace: providerNamespace,
			Operation: OperationPost,
		}
		opts = append(opts, pkgauth.WithCache(*tokenCache, involvedObject))
	}

	token, err := pkgauth.GetToken(ctx, azure.Provider{}, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to get token for azure event hub: %w", err)
	}

	return token.(*azure.Token).AccessToken.Token, nil
}

// validateAuthOptions checks if the authentication options are valid
func validateAuthOptions(endpointURL, token, serviceAccountName string) error {
	if isSASAuth(endpointURL) {
		if err := validateSASAuth(token, serviceAccountName); err != nil {
			return err
		}
	} else if serviceAccountName != "" && token != "" {
		return fmt.Errorf("serviceAccountName and jwt token authentication cannot be set at the same time")
	}

	return nil
}

// isSASAuth checks if the endpoint URL contains SAS authentication parameters
func isSASAuth(endpointURL string) bool {
	return strings.Contains(endpointURL, "SharedAccessKey")
}

// validateSASAuth checks if SAS authentication is used correctly
func validateSASAuth(token, serviceAccountName string) error {
	if serviceAccountName != "" {
		return fmt.Errorf("serviceAccountName and SAS authentication cannot be set at the same time")
	}
	if token != "" {
		return fmt.Errorf("jwt token and SAS authentication cannot be set at the same time")
	}

	return nil
}
