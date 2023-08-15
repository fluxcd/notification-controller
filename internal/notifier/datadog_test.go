package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/stretchr/testify/require"
)

func TestDataDogPost(t *testing.T) {
	thisRun := func(expectedToFail bool) func(t *testing.T) {
		return func(t *testing.T) {
			ddApiKey := "sdfsdf"
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				var payload datadogV1.EventCreateRequest
				err = json.Unmarshal(b, &payload)
				require.NoError(t, err)
				if expectedToFail {
					w.WriteHeader(http.StatusForbidden)
				}
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()

			dd, err := NewDataDog(ts.URL, "", nil, ddApiKey)
			require.NoError(t, err)

			err = dd.Post(context.Background(), testEvent())
			if expectedToFail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}
	}
	t.Run("working", thisRun(false))
	t.Run("failing", thisRun(true))
}

func TestDataDogProviderErrors(t *testing.T) {
	_, err := NewDataDog("https://api.datadoghq.com", "", nil, "")
	require.Error(t, err)
	require.Equal(t, "token cannot be empty", err.Error())

	_, err = NewDataDog("https://bad url :)", "", nil, "token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse address")
}

func TestToDataDogTags(t *testing.T) {
	dd, err := NewDataDog("https://api.datadoghq.com", "", nil, "token")
	require.NoError(t, err)

	event := testEvent()

	tags := dd.toDataDogTags(&event)

	require.Contains(t, tags, "test:metadata")
	require.Contains(t, tags, fmt.Sprintf("kube_kind:%s", strings.ToLower(event.InvolvedObject.Kind)))
	require.Contains(t, tags, fmt.Sprintf("kube_namespace:%s", event.InvolvedObject.Namespace))
	require.Contains(t, tags, fmt.Sprintf("kube_name:%s", strings.ToLower(event.InvolvedObject.Name)))
	require.Contains(t, tags, fmt.Sprintf("flux_reporting_controller:%s", strings.ToLower(event.ReportingController)))
	require.Contains(t, tags, fmt.Sprintf("flux_reason:%s", strings.ToLower(event.Reason)))

}
