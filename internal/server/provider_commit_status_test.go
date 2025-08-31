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

package server

import (
	"context"
	"testing"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
)

func Test_newCommitStatus(t *testing.T) {
	tests := []struct {
		expression   string
		notification *eventv1.Event
		alert        *apiv1beta3.Alert
		provider     *apiv1beta3.Provider
		wantResult   string
		wantErr      bool
	}{
		{
			expression: "event.involvedObject.kind + '/' + event.involvedObject.name + '/' + event.metadata.environment + '/' + provider.metadata.uid + '/' + alert.metadata.name",
			notification: &eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					"environment": "production",
				},
			},
			alert: &apiv1beta3.Alert{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-alert",
				},
			},
			provider: &apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					UID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				},
			},
			wantResult: "Kustomization/gitops-system/production/0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a/test-alert",
		},
		{
			expression: "(event.involvedObject.kind + '/' + event.involvedObject.name + '/' + event.metadata.environment + '/' + provider.metadata.uid + '/' + alert.metadata.name).lowerAscii()",
			notification: &eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					"environment": "production",
				},
			},
			alert: &apiv1beta3.Alert{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-alert",
				},
			},
			provider: &apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					UID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				},
			},
			wantResult: "kustomization/gitops-system/production/0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a/test-alert",
		},
		{
			expression: "event.involvedObject.kind + '/' + event.involvedObject.name + '/' + notification.metadata.name",
			notification: &eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					"environment": "production",
				},
			},
			alert: &apiv1beta3.Alert{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-alert",
				},
			},
			provider: &apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					UID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				},
			},
			wantErr: true,
		},
		{
			expression: "kustomization.metadata.name + '/' + provider.metadata.uid",
			notification: &eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
			},
			provider: &apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					UID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				},
			},
			alert: &apiv1beta3.Alert{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-alert",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(apiv1beta3.AddToScheme(scheme)).ToNot(HaveOccurred())
			g.Expect(corev1.AddToScheme(scheme)).ToNot(HaveOccurred())

			result, err := newCommitStatus(context.Background(), tt.expression, tt.notification, tt.alert, tt.provider)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			g.Expect(result).To(Equal(tt.wantResult))
		})
	}
}

func Test_generateDefaultCommitStatus(t *testing.T) {
	statusIDTests := []struct {
		name        string
		providerUID string
		event       eventv1.Event
		want        string
	}{
		{
			name:        "simple event case",
			providerUID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
			event: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Reason: "ApplySucceeded",
			},
			want: "kustomization/gitops-system/0c9c2e41",
		},
	}

	for _, tt := range statusIDTests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			id := generateDefaultCommitStatus(tt.providerUID, tt.event)

			g.Expect(id).To(Equal(tt.want))
		})
	}
}
