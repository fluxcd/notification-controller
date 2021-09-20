/*
Copyright 2020 The Flux authors

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

	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/fluxcd/pkg/apis/meta"

	"github.com/fluxcd/notification-controller/api/v1beta1"
)

// ReceiverReconciler reconciles a Receiver object
type ReceiverReconciler struct {
	client.Client
	helper.Metrics
	Scheme *runtime.Scheme
}

type ReceiverReconcilerOptions struct {
	MaxConcurrentReconciles int
}

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=receivers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=receivers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=buckets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=buckets/status,verbs=get
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=gitrepositories,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=gitrepositories/status,verbs=get
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=helmrepositories,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=source.fluxcd.io,resources=helmrepositories/status,verbs=get
// +kubebuilder:rbac:groups=image.fluxcd.io,resources=imagerepositories,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=image.fluxcd.io,resources=imagerepositories/status,verbs=get
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ReceiverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	start := time.Now()
	log := ctrl.LoggerFrom(ctx)

	receiver := &v1beta1.Receiver{}
	if err := r.Get(ctx, req.NamespacedName, receiver); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// record suspension metrics
	defer r.RecordSuspend(ctx, receiver, receiver.Spec.Suspend)
	// return early if the object is suspended
	if receiver.Spec.Suspend {
		log.Info("Reconciliation is suspended for this object")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(receiver, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		// Patch the object, ignoring conflicts on the conditions owned by this controller
		patchOpts := []patch.Option{
			patch.WithOwnedConditions{
				Conditions: []string{
					meta.ReadyCondition,
					meta.ReconcilingCondition,
					meta.StalledCondition,
				},
			},
		}

		// Determine if the resource is still being reconciled, or if it has stalled, and record this observation
		if retErr == nil && (result.IsZero() || !result.Requeue) {
			// We are no longer reconciling
			conditions.Delete(receiver, meta.ReconcilingCondition)

			// We have now observed this generation
			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})

			readyCondition := conditions.Get(receiver, meta.ReadyCondition)
			switch readyCondition.Status {
			case metav1.ConditionFalse:
				// As we are no longer reconciling and the end-state is not ready, the reconciliation has stalled
				conditions.MarkStalled(receiver, readyCondition.Reason, readyCondition.Message)
			case metav1.ConditionTrue:
				// As we are no longer reconciling and the end-state is ready, the reconciliation is no longer stalled
				conditions.Delete(receiver, meta.StalledCondition)
			}
		}

		// Finally, patch the resource
		if err := patchHelper.Patch(ctx, receiver, patchOpts...); err != nil {
			retErr = errors.NewAggregate([]error{retErr, err})
		}

		// Always record readiness and duration metrics
		r.Metrics.RecordReadiness(ctx, receiver)
		r.Metrics.RecordDuration(ctx, receiver, start)

	}()

	return r.reconcile(ctx, receiver)
}

func (r *ReceiverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, ReceiverReconcilerOptions{})
}

func (r *ReceiverReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts ReceiverReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Receiver{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

/// reconcile steps through the actual reconciliation tasks for the object, it returns early on the first step that
// produces an error.
func (r *ReceiverReconciler) reconcile(ctx context.Context, obj *v1beta1.Receiver) (ctrl.Result, error) {
	// Mark the resource as under reconciliation
	conditions.MarkReconciling(obj, meta.ProgressingReason, "")

	token, err := r.token(ctx, obj)
	if err != nil {
		conditions.MarkFalse(obj, meta.ReadyCondition, v1beta1.TokenNotFoundReason, err.Error())
		return ctrl.Result{}, err
	}

	receiverURL := fmt.Sprintf("/hook/%s", sha256sum(token+obj.Name+obj.Namespace))

	// Nothing has changed so return early
	if obj.Status.URL == receiverURL && obj.Status.ObservedGeneration == obj.Generation {
		return ctrl.Result{}, nil
	}

	// Mark the resource as ready and set the URL
	conditions.MarkTrue(obj, meta.ReadyCondition, v1beta1.InitializedReason, "Receiver initialised with URL: "+receiverURL,
		receiverURL)
	obj.Status.URL = receiverURL

	ctrl.LoggerFrom(ctx).Info("Receiver initialized")

	return ctrl.Result{}, nil
}

// token extract the token value from the secret object
func (r *ReceiverReconciler) token(ctx context.Context, receiver *v1beta1.Receiver) (string, error) {
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
