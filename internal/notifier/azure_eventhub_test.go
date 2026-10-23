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
	"errors"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
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
			name:              "static JWT authentication is rejected",
			endpointURL:       "azure-nc-eventhub",
			token:             "jwt-token",
			eventHubNamespace: "namespace",
			err:               errors.New("invalid authentication options: static JWT authentication with the 'token' key is no longer supported"),
		},
		{
			name:              "SAS Authentication",
			endpointURL:       "Endpoint=sb://example.com/;SharedAccessKeyName=keyName;SharedAccessKey=key;EntityPath=eventhub",
			token:             "",
			eventHubNamespace: "namespace",
		},
		{
			name:              "SAS Authentication without entity path",
			endpointURL:       "Endpoint=sb://example.com/;SharedAccessKeyName=keyName;SharedAccessKey=key",
			token:             "",
			eventHubNamespace: "namespace",
			err:               errors.New("failed to create a eventhub using SAS: connection string does not contain an EntityPath. eventHub cannot be an empty string"),
		},
		{
			name:              "Default Azure Credential",
			endpointURL:       "azure-nc-eventhub",
			token:             "",
			eventHubNamespace: "namespace",
			err:               errors.New("failed to create a eventhub using managed identity: failed to get token: failed to create provider access credential for the controller: ManagedIdentityCredential"),
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
			err:               errors.New("invalid authentication options: static JWT authentication with the 'token' key is no longer supported"),
		},
		{
			name:               "token auth with serviceAccountName set",
			endpointURL:        "azure-nc-eventhub",
			token:              "test-token",
			serviceAccountName: "test-service-account",
			eventHubNamespace:  "namespace",
			err:                errors.New("invalid authentication options: static JWT authentication with the 'token' key is no longer supported"),
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
			g := NewWithT(t)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			client, err := NewAzureEventHub(ctx, tt.endpointURL, tt.token, tt.eventHubNamespace, nil, tt.serviceAccountName, "", "", nil, nil, nil, "")
			if tt.err != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(ContainSubstring(tt.err.Error())))
				if strings.HasPrefix(tt.err.Error(), "invalid authentication options:") {
					g.Expect(errors.Unwrap(err)).ToNot(BeNil())
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(client).ToNot(BeNil())
			}
		})
	}
}
