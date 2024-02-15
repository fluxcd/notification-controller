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

package controller

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta4 "github.com/fluxcd/notification-controller/api/v1beta4"
)

func getAlertWithFinalizers(finalizers []string) *apiv1beta4.Alert {
	return &apiv1beta4.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Finalizers: finalizers,
		},
	}
}

func TestFinalizerPredicateCreate(t *testing.T) {
	tests := []struct {
		name   string
		object client.Object
		want   bool
	}{
		{
			name:   "no finalizer",
			object: getAlertWithFinalizers([]string{}),
			want:   false,
		},
		{
			name:   "no flux finalizer",
			object: getAlertWithFinalizers([]string{"foo.bar", "baz.bar"}),
			want:   false,
		},
		{
			name:   "has flux finalizer",
			object: getAlertWithFinalizers([]string{"foo.bar", apiv1.NotificationFinalizer, "baz.bar"}),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			event := event.CreateEvent{
				Object: tt.object,
			}

			mp := finalizerPredicate{}
			g.Expect(mp.Create(event)).To(Equal(tt.want))
		})
	}
}

func TestFinalizerPredicateDelete(t *testing.T) {
	tests := []struct {
		name   string
		object client.Object
		want   bool
	}{
		{
			name:   "no finalizer",
			object: getAlertWithFinalizers([]string{}),
			want:   false,
		},
		{
			name:   "no flux finalizer",
			object: getAlertWithFinalizers([]string{"foo.bar", "baz.bar"}),
			want:   false,
		},
		{
			name:   "has flux finalizer",
			object: getAlertWithFinalizers([]string{"foo.bar", apiv1.NotificationFinalizer, "baz.bar"}),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			event := event.DeleteEvent{
				Object: tt.object,
			}

			mp := finalizerPredicate{}
			g.Expect(mp.Delete(event)).To(Equal(tt.want))
		})
	}
}

func TestFinalizerPredicateUpdate(t *testing.T) {
	tests := []struct {
		name      string
		oldObject client.Object
		newObject client.Object
		want      bool
	}{
		{
			name:      "no new object",
			oldObject: getAlertWithFinalizers([]string{apiv1.NotificationFinalizer}),
			newObject: nil,
			want:      false,
		},
		{
			name:      "no old object, new object without flux finalizer",
			oldObject: nil,
			newObject: getAlertWithFinalizers([]string{"foo.bar"}),
			want:      false,
		},
		{
			name:      "no old object, new object with flux finalizer",
			oldObject: nil,
			newObject: getAlertWithFinalizers([]string{apiv1.NotificationFinalizer}),
			want:      true,
		},
		{
			name:      "old and new objects with flux finalizer",
			oldObject: getAlertWithFinalizers([]string{"foo.bar", apiv1.NotificationFinalizer, "baz.bar"}),
			newObject: getAlertWithFinalizers([]string{"foo.bar", apiv1.NotificationFinalizer, "baz.bar"}),
			want:      true,
		},
		{
			name:      "old object with flux finalizer, new object without",
			oldObject: getAlertWithFinalizers([]string{"foo.bar", apiv1.NotificationFinalizer, "baz.bar"}),
			newObject: getAlertWithFinalizers([]string{"foo.bar", "baz.bar"}),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			event := event.UpdateEvent{
				ObjectOld: tt.oldObject,
				ObjectNew: tt.newObject,
			}

			mp := finalizerPredicate{}
			g.Expect(mp.Update(event)).To(Equal(tt.want))
		})
	}
}
