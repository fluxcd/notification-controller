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
	. "github.com/onsi/gomega"
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
			g := NewWithT(t)
			s, err := NewSentry(nil, tt.dsn, tt.environment)
			if tt.err != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(Equal(tt.err))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(s.Client.Options().Dsn).To(Equal(tt.dsn))
				g.Expect(s.Client.Options().Environment).To(Equal(tt.environment))
			}
		})
	}
}

func TestToSentryEvent(t *testing.T) {
	g := NewWithT(t)
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
	g.Expect(s.Timestamp).To(Equal(time.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC)))
	g.Expect(s.Level).To(Equal(sentry.LevelInfo))
	g.Expect(s.ServerName).To(Equal("source-controller"))
	g.Expect(s.Transaction).To(Equal("GitRepository: flux-system/test-app"))
	g.Expect(s.Extra).To(Equal(map[string]interface{}{
		"key1": "val1",
		"key2": "val2",
	}))
	g.Expect(s.Message).To(Equal("message"))
}

func TestToSentrySpan(t *testing.T) {
	g := NewWithT(t)
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
	g.Expect(s.Timestamp).To(Equal(time.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC)))
	g.Expect(s.Type).To(Equal("transaction"))
	g.Expect(s.Tags).To(Equal(map[string]string{
		"flux_involved_object_kind":      e.InvolvedObject.Kind,
		"flux_involved_object_name":      e.InvolvedObject.Name,
		"flux_involved_object_namespace": e.InvolvedObject.Namespace,
		"flux_reason":                    e.Reason,
		"flux_reporting_controller":      e.ReportingController,
		"flux_reporting_instance":        e.ReportingInstance,
		"key1":                           "val1",
		"key2":                           "val2",
	}))
	g.Expect(s.Message).To(Equal("message"))
}
