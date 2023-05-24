/*
Copyright 2023 The Flux authors

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

package server

import (
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"

	apiv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestEnhanceEventWithAlertMetadata(t *testing.T) {
	s := &EventServer{logger: logr.New(nil)}

	for name, tt := range map[string]struct {
		event            eventv1.Event
		alert            apiv1beta2.Alert
		expectedMetadata map[string]string
	}{
		"empty metadata": {
			event:            eventv1.Event{},
			alert:            apiv1beta2.Alert{},
			expectedMetadata: nil,
		},
		"enhanced with summary": {
			event: eventv1.Event{},
			alert: apiv1beta2.Alert{
				Spec: apiv1beta2.AlertSpec{
					Summary: "summary",
				},
			},
			expectedMetadata: map[string]string{
				"summary": "summary",
			},
		},
		"overriden with summary": {
			event: eventv1.Event{
				Metadata: map[string]string{
					"summary": "original summary",
				},
			},
			alert: apiv1beta2.Alert{
				Spec: apiv1beta2.AlertSpec{
					Summary: "summary",
				},
			},
			expectedMetadata: map[string]string{
				"summary": "summary",
			},
		},
		"enhanced with metadata": {
			event: eventv1.Event{},
			alert: apiv1beta2.Alert{
				Spec: apiv1beta2.AlertSpec{
					EventMetadata: map[string]string{
						"foo": "bar",
					},
				},
			},
			expectedMetadata: map[string]string{
				"foo": "bar",
			},
		},
		"skipped override with metadata": {
			event: eventv1.Event{
				Metadata: map[string]string{
					"foo": "baz",
				},
			},
			alert: apiv1beta2.Alert{
				Spec: apiv1beta2.AlertSpec{
					EventMetadata: map[string]string{
						"foo": "bar",
					},
				},
			},
			expectedMetadata: map[string]string{
				"foo": "baz",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			s.enhanceEventWithAlertMetadata(&tt.event, tt.alert)
			g.Expect(tt.event.Metadata).To(BeEquivalentTo(tt.expectedMetadata))
		})
	}
}
