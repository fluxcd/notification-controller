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
	"errors"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewSentry(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		environment string
		err         error
	}{
		{
			name:        "valid DSN",
			dsn:         "https://test@localhost/1",
			environment: "foo",
			err:         nil,
		},
		{
			name:        "empty DSN",
			dsn:         "",
			environment: "foo",
			err:         errors.New("DSN cannot be empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSentry(nil, tt.dsn, tt.environment)
			if tt.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, s.Client.Options().Dsn, tt.dsn)
				assert.Equal(t, s.Client.Options().Environment, tt.environment)
			}
		})
	}
}

func TestToSentryEvent(t *testing.T) {
	// Construct test event
	e := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "GitRepository",
			Namespace: "flux-system",
			Name:      "test-app",
		},
		Severity:  "info",
		Timestamp: metav1.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC),
		Message:   "message",
		Metadata: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		ReportingController: "source-controller",
	}

	// Map to Sentry event
	s := toSentryEvent(e)

	// Assertions
	assert.Equal(t, time.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC), s.Timestamp)
	assert.Equal(t, sentry.LevelInfo, s.Level)
	assert.Equal(t, "source-controller", s.ServerName)
	assert.Equal(t, "GitRepository: flux-system/test-app", s.Transaction)
	assert.Equal(t, map[string]interface{}{
		"key1": "val1",
		"key2": "val2",
	}, s.Extra)
	assert.Equal(t, "message", s.Message)
}

func TestToSentrySpan(t *testing.T) {
	// Construct test event
	e := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "GitRepository",
			Namespace: "flux-system",
			Name:      "test-app",
		},
		Severity:  "info",
		Timestamp: metav1.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC),
		Message:   "message",
		Metadata: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
		ReportingController: "source-controller",
	}

	// Map to Sentry event
	s := eventToSpan(e)

	// Assertions
	assert.Equal(t, time.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC), s.Timestamp)
	assert.Equal(t, "transaction", s.Type)
	assert.Equal(t, map[string]string{
		"flux_involved_object_kind":      e.InvolvedObject.Kind,
		"flux_involved_object_name":      e.InvolvedObject.Name,
		"flux_involved_object_namespace": e.InvolvedObject.Namespace,
		"flux_reason":                    e.Reason,
		"flux_reporting_controller":      e.ReportingController,
		"flux_reporting_instance":        e.ReportingInstance,
		"key1":                           "val1",
		"key2":                           "val2",
	}, s.Tags)
	assert.Equal(t, "message", s.Message)
}
