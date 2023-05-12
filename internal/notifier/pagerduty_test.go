package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
)

const (
	routingKey                = "notARealRoutingKey"
	pagerdutyEUv2EventsAPIURL = "https://events.eu.pagerduty.com"
)

func TestNewPagerDuty(t *testing.T) {
	t.Run("US endpoint", func(t *testing.T) {
		p, err := NewPagerDuty("https://events.pagerduty.com/v2/enqueue", "", nil, routingKey)
		assert.NoError(t, err)
		assert.Equal(t, routingKey, p.RoutingKey)
		assert.NotEqual(t, pagerdutyEUv2EventsAPIURL, p.Endpoint)
	})
	t.Run("EU endpoint", func(t *testing.T) {
		p, err := NewPagerDuty("https://events.eu.pagerduty.com/v2/enqueue", "", nil, routingKey)
		assert.NoError(t, err)
		assert.Equal(t, routingKey, p.RoutingKey)
		assert.Equal(t, pagerdutyEUv2EventsAPIURL, p.Endpoint)
	})
	t.Run("invalid URL", func(t *testing.T) {
		_, err := NewPagerDuty("not a url", "", nil, routingKey)
		assert.Errorf(t, err, "invalid PagerDuty endpoint URL not a url: 'parse \"https://not a url/\": invalid character \" \" in host name'")
	})
}

func TestPagerDutyPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/enqueue", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload pagerduty.V2Event
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)
	})
	mux.HandleFunc("/v2/change/enqueue", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload pagerduty.ChangeEvent
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	pd, err := NewPagerDuty(ts.URL, "", nil, "token")
	require.NoError(t, err)

	err = pd.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}

func TestToPagerDutyV2Event(t *testing.T) {
	// Construct test event
	tests := []struct {
		name string
		e    eventv1.Event
		want pagerduty.V2Event
	}{
		{
			name: "basic",
			e: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind:      "GitRepository",
					Namespace: "flux-system",
					Name:      "test-app",
					UID:       "1234",
				},
				Severity:  "info",
				Timestamp: metav1.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC),
				Message:   "message",
				Reason:    meta.SucceededReason,
				Metadata: map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
				ReportingController: "source-controller",
			},
			want: pagerduty.V2Event{
				RoutingKey: routingKey,
				Action:     "resolve",
				DedupKey:   "1234",
			},
		},
		{
			name: "error",
			e: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind:      "GitRepository",
					Namespace: "flux-system",
					Name:      "test-app",
					UID:       "1234",
				},
				Severity:  "error",
				Timestamp: metav1.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC),
				Message:   "message",
				Reason:    meta.FailedReason,
				Metadata: map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
				ReportingController: "source-controller",
			},
			want: pagerduty.V2Event{
				RoutingKey: routingKey,
				Action:     "trigger",
				DedupKey:   "1234",
				Payload: &pagerduty.V2Payload{
					Summary:   "failed: gitrepository/test-app",
					Severity:  "error",
					Source:    "Flux source-controller",
					Timestamp: "2020-01-01T00:00:00Z",
					Component: "test-app",
					Group:     "GitRepository",
					Details: map[string]interface{}{
						"message": "message",
						"metadata": map[string]string{
							"key1": "val1",
							"key2": "val2",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toPagerDutyV2Event(tt.e, routingKey)
			if !reflect.DeepEqual(got, tt.want) {
				t.Logf("got Payload: %+v", got.Payload)
				t.Errorf("toPagerDutyV2Event() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestToPagerDutyChangeEvent(t *testing.T) {
	e := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "GitRepository",
			Namespace: "flux-system",
			Name:      "test-app",
			UID:       "1234",
		},
		Severity:  "info",
		Timestamp: metav1.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC),
		Message:   "message",
		Reason:    meta.SucceededReason,
		Metadata: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		ReportingController: "source-controller",
	}
	want := pagerduty.ChangeEvent{
		RoutingKey: routingKey,
		Payload: pagerduty.ChangeEventPayload{
			Summary:   "succeeded: gitrepository/test-app",
			Source:    "Flux source-controller",
			Timestamp: "2020-01-01T00:00:00Z",
			CustomDetails: map[string]interface{}{
				"message": "message",
				"metadata": map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
			},
		},
	}
	got := toPagerDutyChangeEvent(e, routingKey)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("toPagerDutyChangeEvent() = %q, want %q", got, want)
	}
}

func TestToPagerDutySeverity(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		want     string
	}{
		{
			name:     "info",
			severity: eventv1.EventSeverityInfo,
			want:     "info",
		},
		{
			name:     "error",
			severity: eventv1.EventSeverityError,
			want:     "error",
		},
		{
			name:     "trace",
			severity: eventv1.EventSeverityTrace,
			want:     "info",
		},
		{
			name:     "invalid",
			severity: "invalid",
			want:     "error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, toPagerDutySeverity(tt.severity))
		})
	}
}
