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

	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/runtime/patch"

	"github.com/fluxcd/notification-controller/internal/notifier"
)

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create

// ProviderReconciler reconciles a Provider object to migrate it to static
// Provider.
type ProviderReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	TokenCache *cache.TokenCache
}

func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1beta3.Provider{}, builder.WithPredicates(providerPredicate{})).
		Complete(r)
}

func (r *ProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	obj := &apiv1beta3.Provider{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
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

	// Examine if the object is under deletion.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(obj)
	}

	// Add finalizer if it doesn't exist.
	if !controllerutil.ContainsFinalizer(obj, apiv1.NotificationFinalizer) {
		controllerutil.AddFinalizer(obj, apiv1.NotificationFinalizer)
	}

	return
}

// reconcileDelete handles the deletion of the object.
// It cleans up the caches and removes the finalizer.
func (r *ProviderReconciler) reconcileDelete(obj *apiv1beta3.Provider) (ctrl.Result, error) {
	// Remove our finalizer from the list
	controllerutil.RemoveFinalizer(obj, apiv1.NotificationFinalizer)

	// Cleanup caches.
	r.TokenCache.DeleteEventsForObject(apiv1beta3.ProviderKind,
		obj.GetName(), obj.GetNamespace(), notifier.OperationPost)

	// Stop reconciliation as the object is being deleted
	return ctrl.Result{}, nil
}
