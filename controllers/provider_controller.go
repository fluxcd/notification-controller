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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/metrics"
	"github.com/fluxcd/pkg/runtime/predicates"

	"github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/internal/notifier"
)

// ProviderReconciler reconciles a Provider object
type ProviderReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	MetricsRecorder *metrics.Recorder
}

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers/status,verbs=get;update;patch

func (r *ProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reconcileStart := time.Now()
	log := logr.FromContext(ctx)

	var provider v1beta1.Provider
	if err := r.Get(ctx, req.NamespacedName, &provider); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// record reconciliation duration
	if r.MetricsRecorder != nil {
		objRef, err := reference.GetReference(r.Scheme, &provider)
		if err != nil {
			return ctrl.Result{}, err
		}
		defer r.MetricsRecorder.RecordDuration(*objRef, reconcileStart)
	}

	// validate provider spec and credentials
	if err := r.validate(ctx, provider); err != nil {
		meta.SetResourceCondition(&provider, meta.ReadyCondition, metav1.ConditionFalse, meta.ReconciliationFailedReason, err.Error())
		if err := r.patchStatus(ctx, req, provider.Status); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{Requeue: true}, err
	}

	if !apimeta.IsStatusConditionTrue(provider.Status.Conditions, meta.ReadyCondition) {
		meta.SetResourceCondition(&provider, meta.ReadyCondition, metav1.ConditionTrue, v1beta1.InitializedReason, v1beta1.InitializedReason)
		if err := r.patchStatus(ctx, req, provider.Status); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		log.Info("Provider initialised")
	}

	r.recordReadiness(ctx, provider)

	return ctrl.Result{}, nil
}

func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Provider{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{})).
		Complete(r)
}

func (r *ProviderReconciler) validate(ctx context.Context, provider v1beta1.Provider) error {
	address := provider.Spec.Address
	token := ""
	if provider.Spec.SecretRef != nil {
		var secret corev1.Secret
		secretName := types.NamespacedName{Namespace: provider.Namespace, Name: provider.Spec.SecretRef.Name}

		if err := r.Get(ctx, secretName, &secret); err != nil {
			return fmt.Errorf("failed to read secret, error: %w", err)
		}

		if a, ok := secret.Data["address"]; ok {
			address = string(a)
		}

		if t, ok := secret.Data["token"]; ok {
			token = string(t)
		}
	}

	if address == "" {
		return fmt.Errorf("no address found in 'spec.address' nor in `spec.secretRef`")
	}

	factory := notifier.NewFactory(address, provider.Spec.Proxy, provider.Spec.Username, provider.Spec.Channel, token)
	if _, err := factory.Notifier(provider.Spec.Type); err != nil {
		return fmt.Errorf("failed to initialise provider, error: %w", err)
	}

	return nil
}

func (r *ProviderReconciler) recordReadiness(ctx context.Context, provider v1beta1.Provider) {
	log := logr.FromContext(ctx)
	if r.MetricsRecorder == nil {
		return
	}

	objRef, err := reference.GetReference(r.Scheme, &provider)
	if err != nil {
		log.Error(err, "unable to record readiness metric")
		return
	}
	if rc := apimeta.FindStatusCondition(provider.Status.Conditions, meta.ReadyCondition); rc != nil {
		r.MetricsRecorder.RecordCondition(*objRef, *rc, !provider.DeletionTimestamp.IsZero())
	} else {
		r.MetricsRecorder.RecordCondition(*objRef, metav1.Condition{
			Type:   meta.ReadyCondition,
			Status: metav1.ConditionUnknown,
		}, !provider.DeletionTimestamp.IsZero())
	}
}

func (r *ProviderReconciler) patchStatus(ctx context.Context, req ctrl.Request, newStatus v1beta1.ProviderStatus) error {
	var provider v1beta1.Provider
	if err := r.Get(ctx, req.NamespacedName, &provider); err != nil {
		return err
	}

	patch := client.MergeFrom(provider.DeepCopy())
	provider.Status = newStatus

	return r.Status().Patch(ctx, &provider, patch)
}
