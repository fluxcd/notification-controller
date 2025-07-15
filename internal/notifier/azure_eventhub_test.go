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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAzureEventHub(t *testing.T) {
	tests := []struct {
		name               string
		endpointURL        string
		token              string
		eventHubNamespace  string
		serviceAccountName string
		err                error
	}{
		{
			name:              "JWT Authentication",
			endpointURL:       "azure-nc-eventhub",
			token:             "jwt-token",
			eventHubNamespace: "namespace",
		},
		{
			name:              "SAS Authentication",
			endpointURL:       "Endpoint=sb://example.com/;SharedAccessKeyName=keyName;SharedAccessKey=key;EntityPath=eventhub",
			token:             "",
			eventHubNamespace: "namespace",
		},
		{
			name:              "Default Azure Credential",
			endpointURL:       "azure-nc-eventhub",
			token:             "",
			eventHubNamespace: "namespace",
			err:               errors.New("failed to create a eventhub using managed identity failed to get token for azure event hub: failed to create provider access token for the controller: ManagedIdentityCredential: failed to authenticate a system assigned identity. The endpoint responded with {\"error\":\"invalid_request\",\"error_description\":\"Identity not found\"}"),
		},
		{
			name:               "SAS auth with serviceAccountName set",
			endpointURL:        "Endpoint=sb://example.com/;SharedAccessKeyName=keyName;SharedAccessKey=key;EntityPath=eventhub",
			token:              "",
			serviceAccountName: "test-service-account",
			eventHubNamespace:  "namespace",
			err:                errors.New("invalid authentication options: serviceAccountName and SAS authentication cannot be set at the same time"),
		},
		{
			name:              "SAS auth with token set",
			endpointURL:       "Endpoint=sb://example.com/;SharedAccessKeyName=keyName;SharedAccessKey=key;EntityPath=eventhub",
			token:             "test-token",
			eventHubNamespace: "namespace",
			err:               errors.New("invalid authentication options: jwt token and SAS authentication cannot be set at the same time"),
		},
		{
			name:               "token auth with serviceAccountName set",
			endpointURL:        "azure-nc-eventhub",
			token:              "test-token",
			serviceAccountName: "test-service-account",
			eventHubNamespace:  "namespace",
			err:                errors.New("invalid authentication options: serviceAccountName and jwt token authentication cannot be set at the same time"),
		},
		{
			name:              "empty endpoint URL",
			endpointURL:       "",
			token:             "test-token",
			eventHubNamespace: "namespace",
			err:               errors.New("invalid authentication options: endpoint URL cannot be empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewAzureEventHub(context.TODO(), tt.endpointURL, tt.token, tt.eventHubNamespace, "", tt.serviceAccountName, "", "", nil, nil)
			if tt.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.err, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}
