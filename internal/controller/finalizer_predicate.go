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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

// finalizerPredicate implements predicate functions to allow events for objects
// that have the flux finalizer.
type finalizerPredicate struct {
	predicate.Funcs
}

// Create allows events for objects with flux finalizer that have beed created.
func (finalizerPredicate) Create(e event.CreateEvent) bool {
	return controllerutil.ContainsFinalizer(e.Object, apiv1.NotificationFinalizer)
}

// Update allows events for objects with flux finalizer that have been updated.
func (finalizerPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectNew == nil {
		return false
	}
	return controllerutil.ContainsFinalizer(e.ObjectNew, apiv1.NotificationFinalizer)
}

// Delete allows events for objects with flux finalizer that have been marked
// for deletion.
func (finalizerPredicate) Delete(e event.DeleteEvent) bool {
	return controllerutil.ContainsFinalizer(e.Object, apiv1.NotificationFinalizer)
}
