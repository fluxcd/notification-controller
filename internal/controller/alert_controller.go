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
	"context"

	corev1 "k8s.io/api/core/v1"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	"github.com/fluxcd/pkg/runtime/patch"
)

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=alerts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// AlertReconciler reconciles an Alert object to migrate it to static Alert.
type AlertReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	ControllerName string
}

func (r *AlertReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1beta3.Alert{}, builder.WithPredicates(finalizerPredicate{})).
		Complete(r)
}

func (r *AlertReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)

	obj := &apiv1beta3.Alert{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Early return if no migration is needed.
	if !controllerutil.ContainsFinalizer(obj, apiv1.NotificationFinalizer) {
		return ctrl.Result{}, nil
	}

	// Examine if the object is under deletion.
	var delete bool
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		delete = true
	}

	// Skip if it's suspend and not being deleted.
	if obj.Spec.Suspend && !delete {
		log.Info("reconciliation is suspended for this object")
		return ctrl.Result{}, nil
	}

	patcher, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if err := patcher.Patch(ctx, obj); err != nil {
			retErr = err
		}
	}()

	// Remove the notification-controller finalizer.
	controllerutil.RemoveFinalizer(obj, apiv1.NotificationFinalizer)

	log.Info("removed finalizer from Alert to migrate to static Alert")
	r.Event(obj, corev1.EventTypeNormal, "Migration", "removed finalizer from Alert to migrate to static Alert")

	return
}
