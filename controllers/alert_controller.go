/*
Copyright 2022 The Flux authors

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

package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/predicates"
	kuberecorder "k8s.io/client-go/tools/record"

	apiv1 "github.com/fluxcd/notification-controller/api/v1beta2"
)

var (
	ProviderIndexKey = ".metadata.provider"
)

// AlertReconciler reconciles a Alert object
type AlertReconciler struct {
	client.Client
	helper.Metrics
	kuberecorder.EventRecorder

	ControllerName string
}

type AlertReconcilerOptions struct {
	MaxConcurrentReconciles int
	RateLimiter             ratelimiter.RateLimiter
}

func (r *AlertReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, AlertReconcilerOptions{})
}

func (r *AlertReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts AlertReconcilerOptions) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &apiv1.Alert{}, ProviderIndexKey,
		func(o client.Object) []string {
			alert := o.(*apiv1.Alert)
			return []string{
				fmt.Sprintf("%s/%s", alert.GetNamespace(), alert.Spec.ProviderRef.Name),
			}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Alert{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{}),
		)).
		Watches(
			&source.Kind{Type: &apiv1.Provider{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForProviderChange),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: opts.MaxConcurrentReconciles,
			RateLimiter:             opts.RateLimiter,
			RecoverPanic:            true,
		}).
		Complete(r)
}

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=alerts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=alerts/status,verbs=get;update;patch

func (r *AlertReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	reconcileStart := time.Now()
	log := ctrl.LoggerFrom(ctx)

	obj := &apiv1.Alert{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize the runtime patcher with the current version of the object.
	patcher := patch.NewSerialPatcher(obj, r.Client)

	defer func() {
		// Patch finalizers, status and conditions.
		if err := r.patch(ctx, obj, patcher); err != nil {
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}

		// Record Prometheus metrics.
		r.Metrics.RecordReadiness(ctx, obj)
		r.Metrics.RecordDuration(ctx, obj, reconcileStart)
		r.Metrics.RecordSuspend(ctx, obj, obj.Spec.Suspend)

		// Emit warning event if the reconciliation failed.
		if retErr != nil {
			r.Event(obj, corev1.EventTypeWarning, meta.FailedReason, retErr.Error())
		}

		// Log and emit success event.
		if retErr == nil && conditions.IsReady(obj) {
			msg := "Reconciliation finished"
			log.Info(msg)
			r.Event(obj, corev1.EventTypeNormal, meta.SucceededReason, msg)
		}
	}()

	if !controllerutil.ContainsFinalizer(obj, apiv1.NotificationFinalizer) {
		controllerutil.AddFinalizer(obj, apiv1.NotificationFinalizer)
		result = ctrl.Result{Requeue: true}
		return
	}

	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(obj, apiv1.NotificationFinalizer)
		result = ctrl.Result{}
		return
	}

	// Return early if the object is suspended.
	if obj.Spec.Suspend {
		log.Info("Reconciliation is suspended for this object")
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, obj)
}

func (r *AlertReconciler) reconcile(ctx context.Context, alert *apiv1.Alert) (ctrl.Result, error) {
	// Mark the resource as under reconciliation.
	conditions.MarkReconciling(alert, meta.ProgressingReason, "Reconciliation in progress")

	// Check if the provider exist and is ready.
	if err := r.isProviderReady(ctx, alert); err != nil {
		conditions.MarkFalse(alert, meta.ReadyCondition, meta.FailedReason, err.Error())
		return ctrl.Result{Requeue: true}, client.IgnoreNotFound(err)
	}

	conditions.MarkTrue(alert, meta.ReadyCondition, meta.SucceededReason, apiv1.InitializedReason)

	return ctrl.Result{}, nil
}

func (r *AlertReconciler) isProviderReady(ctx context.Context, alert *apiv1.Alert) error {
	provider := &apiv1.Provider{}
	providerName := types.NamespacedName{Namespace: alert.Namespace, Name: alert.Spec.ProviderRef.Name}
	if err := r.Get(ctx, providerName, provider); err != nil {
		// log not found errors since they get filtered out
		ctrl.LoggerFrom(ctx).Error(err, "failed to get provider %s", providerName.String())
		return fmt.Errorf("failed to get provider '%s': %w", providerName.String(), err)
	}

	if !conditions.IsReady(provider) {
		return fmt.Errorf("provider %s is not ready", providerName.String())
	}

	return nil
}

func (r *AlertReconciler) requestsForProviderChange(o client.Object) []reconcile.Request {
	provider, ok := o.(*apiv1.Provider)
	if !ok {
		panic(fmt.Errorf("expected a provider, got %T", o))
	}

	ctx := context.Background()
	var list apiv1.AlertList
	if err := r.List(ctx, &list, client.MatchingFields{
		ProviderIndexKey: client.ObjectKeyFromObject(provider).String(),
	}); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	for _, i := range list.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&i)})
	}

	return reqs
}

// patch updates the object status, conditions and finalizers.
func (r *AlertReconciler) patch(ctx context.Context, obj *apiv1.Alert, patcher *patch.SerialPatcher) (retErr error) {
	// Configure the runtime patcher.
	patchOpts := []patch.Option{}
	ownedConditions := []string{
		meta.ReadyCondition,
		meta.ReconcilingCondition,
		meta.StalledCondition,
	}
	patchOpts = append(patchOpts,
		patch.WithOwnedConditions{Conditions: ownedConditions},
		patch.WithForceOverwriteConditions{},
		patch.WithFieldOwner(r.ControllerName),
	)

	// Set the value of the reconciliation request in status.
	if v, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		obj.Status.LastHandledReconcileAt = v
	}

	// Remove the Reconciling condition and update the observed generation
	// if the reconciliation was successful.
	if conditions.IsTrue(obj, meta.ReadyCondition) {
		conditions.Delete(obj, meta.ReconcilingCondition)
		obj.Status.ObservedGeneration = obj.Generation
	}

	// Set the Reconciling reason to ProgressingWithRetry if the
	// reconciliation has failed.
	if conditions.IsFalse(obj, meta.ReadyCondition) &&
		conditions.Has(obj, meta.ReconcilingCondition) {
		rc := conditions.Get(obj, meta.ReconcilingCondition)
		rc.Reason = apiv1.ProgressingWithRetryReason
		conditions.Set(obj, rc)
	}

	// Patch the object status, conditions and finalizers.
	if err := patcher.Patch(ctx, obj, patchOpts...); err != nil {
		if !obj.GetDeletionTimestamp().IsZero() {
			err = kerrors.FilterOut(err, func(e error) bool { return apierrors.IsNotFound(e) })
		}
		retErr = kerrors.NewAggregate([]error{retErr, err})
		if retErr != nil {
			return retErr
		}
	}

	return nil
}
