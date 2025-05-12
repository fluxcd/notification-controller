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

package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
)

// providerPredicate implements predicate functions for the Provider API.
type providerPredicate struct{}

func (providerPredicate) Create(e event.CreateEvent) bool {
	return !controllerutil.ContainsFinalizer(e.Object, apiv1.NotificationFinalizer)
}

func (providerPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectNew == nil {
		return false
	}
	return !controllerutil.ContainsFinalizer(e.ObjectNew, apiv1.NotificationFinalizer) ||
		!e.ObjectNew.(*apiv1beta3.Provider).ObjectMeta.DeletionTimestamp.IsZero()
}

func (providerPredicate) Delete(e event.DeleteEvent) bool {
	return false
}

func (providerPredicate) Generic(e event.GenericEvent) bool {
	return !controllerutil.ContainsFinalizer(e.Object, apiv1.NotificationFinalizer)
}
