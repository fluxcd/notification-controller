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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMSTeams(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantSchema int
	}{
		{
			name:       "deprecated connector url",
			url:        "https://xxx.webhook.office.com",
			wantSchema: msTeamsSchemaDeprecatedConnector,
		},
		{
			name:       "deprecated connector url with port",
			url:        "https://xxx.webhook.office.com:443",
			wantSchema: msTeamsSchemaDeprecatedConnector,
		},
		{
			name:       "url close to deprecated connector url but different",
			url:        "https://xxx-webhook.office.com",
			wantSchema: msTeamsSchemaAdaptiveCard,
		},
		{
			name:       "incoming webhook workflow url",
			url:        "https://prod-28.northeurope.logic.azure.com:443/workflows/xxx/triggers/manual/paths/invoke",
			wantSchema: msTeamsSchemaAdaptiveCard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			teams, err := NewMSTeams(tt.url, "", nil)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSchema, teams.Schema)
		})
	}
}

func TestMSTeams_Post(t *testing.T) {
	var deprecatedConnectorCalled bool
	deprecatedConnectorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deprecatedConnectorCalled = true

		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload = MSTeamsPayload{}
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		require.Equal(t, "gitrepository/webapp.gitops-system", payload.Sections[0].ActivitySubtitle)
		require.Equal(t, "metadata", payload.Sections[0].Facts[0].Value)
	}))
	defer deprecatedConnectorServer.Close()

	var adaptiveCardCalled bool
	adaptiveCardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adaptiveCardCalled = true

		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload map[string]any
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		assert.Equal(t, map[string]any{
			"type": "message",
			"attachments": []any{
				map[string]any{
					"contentType": "application/vnd.microsoft.card.adaptive",
					"content": map[string]any{
						"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
						"type":    "AdaptiveCard",
						"version": "1.4",
						"body": []any{
							map[string]any{
								"type": "Container",
								"items": []any{
									map[string]any{
										"type":   "TextBlock",
										"size":   "large",
										"text":   "gitrepository/webapp.gitops-system",
										"weight": "bolder",
										"wrap":   true,
									},
									map[string]any{
										"type": "TextBlock",
										"text": "message",
										"wrap": true,
									},
									map[string]any{
										"type": "FactSet",
										"facts": []any{
											map[string]any{
												"title": "test",
												"value": "metadata",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}, payload)
	}))
	defer adaptiveCardServer.Close()

	tests := []struct {
		name         string
		url          string
		schema       int
		serverCalled *bool
	}{
		{
			name:         "deprecated connector",
			url:          deprecatedConnectorServer.URL,
			schema:       msTeamsSchemaDeprecatedConnector,
			serverCalled: &deprecatedConnectorCalled,
		},
		{
			name:         "adaptive card",
			url:          adaptiveCardServer.URL,
			schema:       msTeamsSchemaAdaptiveCard,
			serverCalled: &adaptiveCardCalled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			*tt.serverCalled = false

			teams, err := NewMSTeams(tt.url, "", nil)
			require.NoError(t, err)
			teams.Schema = tt.schema

			err = teams.Post(context.TODO(), testEvent())
			require.NoError(t, err)

			assert.True(t, *tt.serverCalled)
		})
	}
}
