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
	"crypto/sha256"
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
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/predicates"

	apiv1 "github.com/fluxcd/notification-controller/api/v1beta2"
)

// ReceiverReconciler reconciles a Receiver object
type ReceiverReconciler struct {
	client.Client
	helper.Metrics

	ControllerName string
}

type ReceiverReconcilerOptions struct {
	MaxConcurrentReconciles int
	RateLimiter             ratelimiter.RateLimiter
}

func (r *ReceiverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, ReceiverReconcilerOptions{})
}

func (r *ReceiverReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts ReceiverReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Receiver{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{}),
		)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: opts.MaxConcurrentReconciles,
			RateLimiter:             opts.RateLimiter,
			RecoverPanic:            true,
		}).
		Complete(r)
}

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=receivers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=receivers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=buckets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=buckets/status,verbs=get
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=gitrepositories,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=gitrepositories/status,verbs=get
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=ocirepositories,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=ocirepositories/status,verbs=get
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=helmrepositories,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=helmrepositories/status,verbs=get
// +kubebuilder:rbac:groups=image.fluxcd.io,resources=imagerepositories,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=image.fluxcd.io,resources=imagerepositories/status,verbs=get
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ReceiverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	reconcileStart := time.Now()
	log := ctrl.LoggerFrom(ctx)

	obj := &apiv1.Receiver{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize the runtime patcher with the current version of the object.
	patcher := patch.NewSerialPatcher(obj, r.Client)

	defer func() {
		// Record Prometheus metrics.
		r.Metrics.RecordReadiness(ctx, obj)
		r.Metrics.RecordDuration(ctx, obj, reconcileStart)
		r.Metrics.RecordSuspend(ctx, obj, obj.Spec.Suspend)

		// Patch finalizers, status and conditions.
		retErr = r.patch(ctx, obj, patcher)
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

// reconcile steps through the actual reconciliation tasks for the object, it returns early on the first step that
// produces an error.
func (r *ReceiverReconciler) reconcile(ctx context.Context, obj *apiv1.Receiver) (ctrl.Result, error) {
	// Mark the resource as under reconciliation
	conditions.MarkReconciling(obj, meta.ProgressingReason, "Reconciliation in progress")

	token, err := r.token(ctx, obj)
	if err != nil {
		conditions.MarkFalse(obj, meta.ReadyCondition, apiv1.TokenNotFoundReason, err.Error())
		return ctrl.Result{Requeue: true}, err
	}

	receiverURL := fmt.Sprintf("/hook/%s", sha256sum(token+obj.Name+obj.Namespace))
	msg := fmt.Sprintf("Receiver initialized with URL: %s", receiverURL)

	// Mark the resource as ready and set the URL
	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, msg)
	obj.Status.URL = receiverURL

	ctrl.LoggerFrom(ctx).Info(msg)

	return ctrl.Result{}, nil
}

// patch updates the object status, conditions and finalizers.
func (r *ReceiverReconciler) patch(ctx context.Context, obj *apiv1.Receiver, patcher *patch.SerialPatcher) (retErr error) {
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

// token extract the token value from the secret object
func (r *ReceiverReconciler) token(ctx context.Context, receiver *apiv1.Receiver) (string, error) {
	token := ""
	secretName := types.NamespacedName{
		Namespace: receiver.GetNamespace(),
		Name:      receiver.Spec.SecretRef.Name,
	}

	var secret corev1.Secret
	err := r.Client.Get(ctx, secretName, &secret)
	if err != nil {
		return "", fmt.Errorf("unable to read token from secret '%s' error: %w", secretName, err)
	}

	if val, ok := secret.Data["token"]; ok {
		token = string(val)
	} else {
		return "", fmt.Errorf("invalid '%s' secret data: required fields 'token'", secretName)
	}

	return token, nil
}

func sha256sum(val string) string {
	digest := sha256.Sum256([]byte(val))
	return fmt.Sprintf("%x", digest)
}
