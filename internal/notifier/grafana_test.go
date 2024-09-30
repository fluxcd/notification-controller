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

func TestGrafana_Post(t *testing.T) {
	t.Run("Successfully post and expect 200 ok", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var payload = GraphitePayload{}
			err = json.Unmarshal(b, &payload)
			require.NoError(t, err)

			require.Equal(t, "gitrepository/webapp.gitops-system", payload.Text)
			require.Equal(t, "flux", payload.Tags[0])
			require.Equal(t, "source-controller", payload.Tags[1])
			require.Equal(t, "test: metadata", payload.Tags[2])
			require.Equal(t, "name: webapp", payload.Tags[3])
			require.Equal(t, "namespace: gitops-system", payload.Tags[4])
		}))
		defer ts.Close()

		grafana, err := NewGrafana(ts.URL, "", "", nil, "", "")
		require.NoError(t, err)

		err = grafana.Post(context.TODO(), testEvent())
		assert.NoError(t, err)
	})
}
