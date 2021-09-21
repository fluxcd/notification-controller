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
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/predicates"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fluxcd/notification-controller/api/v1beta1"
)

var (
	ProviderIndexKey string = ".metadata.provider"
)

// AlertReconciler reconciles a Alert object
type AlertReconciler struct {
	client.Client
	helper.Metrics
	helper.Events

	Scheme *runtime.Scheme
}

type AlertReconcilerOptions struct {
	MaxConcurrentReconciles int
}

func (r *AlertReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, AlertReconcilerOptions{})
}

func (r *AlertReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts AlertReconcilerOptions) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1beta1.Alert{}, ProviderIndexKey,
		func(o client.Object) []string {
			alert := o.(*v1beta1.Alert)
			return []string{
				fmt.Sprintf("%s/%s", alert.GetNamespace(), alert.Spec.ProviderRef.Name),
			}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Alert{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{})).
		Watches(
			&source.Kind{Type: &v1beta1.Provider{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForProviderChange),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=alerts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=alerts/status,verbs=get;update;patch

func (r *AlertReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	start := time.Now()
	log := ctrl.LoggerFrom(ctx)

	alert := &v1beta1.Alert{}
	if err := r.Get(ctx, req.NamespacedName, alert); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// record suspension metrics
	r.RecordSuspend(ctx, alert, alert.Spec.Suspend)

	if alert.Spec.Suspend {
		log.Info("Reconciliation is suspended for this object")
		return ctrl.Result{}, nil
	}

	patchHelper, err := patch.NewHelper(alert, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		patchOpts := []patch.Option{
			patch.WithOwnedConditions{
				Conditions: []string{
					meta.ReadyCondition,
					meta.ReconcilingCondition,
					meta.StalledCondition,
				},
			},
		}

		if retErr == nil && (result.IsZero() || !result.Requeue) {
			conditions.Delete(alert, meta.ReconcilingCondition)

			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})

			readyCondition := conditions.Get(alert, meta.ReadyCondition)
			switch readyCondition.Status {
			case metav1.ConditionFalse:
				// As we are no longer reconciling and the end-state is not ready, the reconciliation has stalled
				conditions.MarkStalled(alert, readyCondition.Reason, readyCondition.Message)
			case metav1.ConditionTrue:
				// As we are no longer reconciling and the end-state is ready, the reconciliation is no longer stalled
				conditions.Delete(alert, meta.StalledCondition)
			}
		}

		if err := patchHelper.Patch(ctx, alert, patchOpts...); err != nil {
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}

		r.Metrics.RecordReadiness(ctx, alert)
		r.Metrics.RecordDuration(ctx, alert, start)
	}()

	return r.reconcile(ctx, alert)
}

func (r *AlertReconciler) reconcile(ctx context.Context, alert *v1beta1.Alert) (ctrl.Result, error) {
	// Mark the resource as under reconciliation
	conditions.MarkReconciling(alert, meta.ProgressingReason, "")

	// validate alert spec and provider
	if err := r.validate(ctx, alert); err != nil {
		conditions.MarkFalse(alert, meta.ReadyCondition, meta.FailedReason, err.Error())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	conditions.MarkTrue(alert, meta.ReadyCondition, meta.SucceededReason, v1beta1.InitializedReason)
	ctrl.LoggerFrom(ctx).Info("Alert initialized")

	return ctrl.Result{}, nil
}

func (r *AlertReconciler) validate(ctx context.Context, alert *v1beta1.Alert) error {
	provider := &v1beta1.Provider{}
	providerName := types.NamespacedName{Namespace: alert.Namespace, Name: alert.Spec.ProviderRef.Name}
	if err := r.Get(ctx, providerName, provider); err != nil {
		// log not found errors since they get filtered out
		ctrl.LoggerFrom(ctx).Error(err, "failed to get provider %s, error: %w", providerName.String())
		return fmt.Errorf("failed to get provider '%s', error: %w", providerName.String(), err)
	}

	if !conditions.IsReady(provider) {
		return fmt.Errorf("provider %s is not ready", providerName.String())
	}

	return nil
}

func (r *AlertReconciler) requestsForProviderChange(o client.Object) []reconcile.Request {
	provider, ok := o.(*v1beta1.Provider)
	if !ok {
		panic(fmt.Errorf("expected a provider, got %T", o))
	}

	ctx := context.Background()
	var list v1beta1.AlertList
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
