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

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/predicates"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	"github.com/fluxcd/notification-controller/internal/server"
)

// ReceiverReconciler reconciles a Receiver object
type ReceiverReconciler struct {
	client.Client
	helper.Metrics
	kuberecorder.EventRecorder

	ControllerName string
}

type ReceiverReconcilerOptions struct {
	RateLimiter           workqueue.TypedRateLimiter[reconcile.Request]
	WatchConfigsPredicate predicate.Predicate
}

func (r *ReceiverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, ReceiverReconcilerOptions{
		WatchConfigsPredicate: predicate.Not(predicate.Funcs{}),
	})
}

const (
	secretRefIndex = ".metadata.secretRef"
)

func (r *ReceiverReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts ReceiverReconcilerOptions) error {
	// This index is used to list Receivers by their webhook path after the receiver server
	// gets a request.
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Receiver{},
		server.WebhookPathIndexKey, server.IndexReceiverWebhookPath); err != nil {
		return err
	}

	// Index receivers by the secret reference, so that we can enqueue
	// Receiver requests when the referenced Secret is changed.
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Receiver{},
		secretRefIndex, func(obj client.Object) []string {
			receiver := obj.(*apiv1.Receiver)
			return []string{fmt.Sprintf("%s/%s", receiver.GetNamespace(), receiver.Spec.SecretRef.Name)}
		}); err != nil {
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Receiver{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{}),
		)).
		WatchesMetadata(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueRequestsForChangeOf),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}, opts.WatchConfigsPredicate),
		).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).
		Complete(r)
}

// enqueueRequestsForChangeOf enqueues Receiver requests for changes in referenced Secret objects.
func (r *ReceiverReconciler) enqueueRequestsForChangeOf(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	// List all Receivers that have the referenced Secret in their spec.
	receivers := &apiv1.ReceiverList{}
	if err := r.List(ctx, receivers, client.MatchingFields{
		secretRefIndex: client.ObjectKeyFromObject(obj).String(),
	}); err != nil {
		log.Error(err, "failed to list Receivers for change of Secret",
			"secretRef", map[string]string{
				"name":      obj.GetName(),
				"namespace": obj.GetNamespace(),
			})
		return nil
	}

	requests := make([]reconcile.Request, 0, len(receivers.Items))
	for _, receiver := range receivers.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      receiver.Name,
				Namespace: receiver.Namespace,
			},
		})
	}
	return requests
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
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
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
		// Patch finalizers, status and conditions.
		if err := r.patch(ctx, obj, patcher); err != nil {
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}

		// Record Prometheus metrics.
		r.Metrics.RecordDuration(ctx, obj, reconcileStart)

		// Emit warning event if the reconciliation failed.
		if retErr != nil {
			r.Event(obj, corev1.EventTypeWarning, meta.FailedReason, retErr.Error())
		}

		// Log and emit success event.
		if retErr == nil && conditions.IsReady(obj) {
			msg := fmt.Sprintf("Reconciliation finished, next run in %s", obj.GetInterval().String())
			log.Info(msg)
			r.Event(obj, corev1.EventTypeNormal, meta.SucceededReason, msg)
		}
	}()

	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(obj, apiv1.NotificationFinalizer)
		result = ctrl.Result{}
		return
	}

	// Add finalizer first if not exist to avoid the race condition
	// between init and delete.
	// Note: Finalizers in general can only be added when the deletionTimestamp
	// is not set.
	if !controllerutil.ContainsFinalizer(obj, apiv1.NotificationFinalizer) {
		controllerutil.AddFinalizer(obj, apiv1.NotificationFinalizer)
		result = ctrl.Result{Requeue: true}
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
	log := ctrl.LoggerFrom(ctx)

	if filter := obj.Spec.ResourceFilter; filter != "" {
		if err := server.ValidateResourceFilter(filter); err != nil {
			const msg = "Reconciliation failed terminally due to configuration error"
			errMsg := fmt.Sprintf("%s: %v", msg, err)
			conditions.MarkFalse(obj, meta.ReadyCondition, meta.InvalidCELExpressionReason, "%s", errMsg)
			conditions.MarkStalled(obj, meta.InvalidCELExpressionReason, "%s", errMsg)
			obj.Status.ObservedGeneration = obj.Generation
			log.Error(err, msg)
			r.Event(obj, corev1.EventTypeWarning, meta.InvalidCELExpressionReason, errMsg)
			return ctrl.Result{}, nil
		}
	}

	// Mark the resource as under reconciliation.
	conditions.MarkReconciling(obj, meta.ProgressingReason, "Reconciliation in progress")

	token, err := r.token(ctx, obj)
	if err != nil {
		conditions.MarkFalse(obj, meta.ReadyCondition, apiv1.TokenNotFoundReason, "%s", err)
		obj.Status.WebhookPath = ""
		return ctrl.Result{}, err
	}

	webhookPath := obj.GetWebhookPath(token)
	msg := fmt.Sprintf("Receiver initialized for path: %s", webhookPath)

	// Mark the resource as ready and set the webhook path in status.
	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "%s", msg)

	if obj.Status.WebhookPath != webhookPath {
		obj.Status.WebhookPath = webhookPath
		log.Info(msg)
	}

	return ctrl.Result{RequeueAfter: obj.GetInterval()}, nil
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
		rc.Reason = meta.ProgressingWithRetryReason
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
