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
	"crypto/x509"
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
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/predicates"

	apiv1 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/notification-controller/internal/notifier"
)

// ProviderReconciler reconciles a Provider object
type ProviderReconciler struct {
	client.Client
	helper.Metrics

	ControllerName string
}

type ProviderReconcilerOptions struct {
	MaxConcurrentReconciles int
	RateLimiter             ratelimiter.RateLimiter
}

func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, ProviderReconcilerOptions{})
}

func (r *ProviderReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts ProviderReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Provider{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{}),
		)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: opts.MaxConcurrentReconciles,
			RateLimiter:             opts.RateLimiter,
			RecoverPanic:            true,
		}).
		Complete(r)
}

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	reconcileStart := time.Now()
	log := ctrl.LoggerFrom(ctx)

	obj := &apiv1.Provider{}
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

func (r *ProviderReconciler) reconcile(ctx context.Context, obj *apiv1.Provider) (ctrl.Result, error) {
	// Mark the resource as under reconciliation
	conditions.MarkReconciling(obj, meta.ProgressingReason, "Reconciliation in progress")

	// validate provider spec and credentials
	if err := r.validate(ctx, obj); err != nil {
		conditions.MarkFalse(obj, meta.ReadyCondition, apiv1.ValidationFailedReason, err.Error())
		return ctrl.Result{Requeue: true}, err
	}

	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, apiv1.InitializedReason)
	ctrl.LoggerFrom(ctx).Info("Provider initialized")

	return ctrl.Result{}, nil
}

func (r *ProviderReconciler) validate(ctx context.Context, provider *apiv1.Provider) error {
	address := provider.Spec.Address
	proxy := provider.Spec.Proxy
	username := provider.Spec.Username
	password := ""
	token := ""
	headers := make(map[string]string)
	if provider.Spec.SecretRef != nil {
		var secret corev1.Secret
		secretName := types.NamespacedName{Namespace: provider.Namespace, Name: provider.Spec.SecretRef.Name}

		if err := r.Get(ctx, secretName, &secret); err != nil {
			return fmt.Errorf("failed to read secret, error: %w", err)
		}

		if a, ok := secret.Data["address"]; ok {
			address = string(a)
		}

		if p, ok := secret.Data["password"]; ok {
			password = string(p)
		}

		if p, ok := secret.Data["proxy"]; ok {
			proxy = string(p)
		}

		if t, ok := secret.Data["token"]; ok {
			token = string(t)
		}

		if u, ok := secret.Data["username"]; ok {
			username = string(u)
		}

		if h, ok := secret.Data["headers"]; ok {
			err := yaml.Unmarshal(h, headers)
			if err != nil {
				return fmt.Errorf("failed to read headers from secret, error: %w", err)
			}
		}
	}

	if address == "" {
		return fmt.Errorf("no address found in 'spec.address' nor in `spec.secretRef`")
	}

	var certPool *x509.CertPool
	if provider.Spec.CertSecretRef != nil {
		var secret corev1.Secret
		secretName := types.NamespacedName{Namespace: provider.Namespace, Name: provider.Spec.CertSecretRef.Name}

		if err := r.Get(ctx, secretName, &secret); err != nil {
			return fmt.Errorf("failed to read secret, error: %w", err)
		}

		caFile, ok := secret.Data["caFile"]
		if !ok {
			return fmt.Errorf("no caFile found in secret %s", provider.Spec.CertSecretRef.Name)
		}

		certPool = x509.NewCertPool()
		ok = certPool.AppendCertsFromPEM(caFile)
		if !ok {
			return fmt.Errorf("could not append to cert pool: invalid CA found in %s", provider.Spec.CertSecretRef.Name)
		}
	}

	factory := notifier.NewFactory(address, proxy, username, provider.Spec.Channel, token, headers, certPool, password, string(provider.UID))
	if _, err := factory.Notifier(provider.Spec.Type); err != nil {
		return fmt.Errorf("failed to initialize provider, error: %w", err)
	}

	return nil
}

// patch updates the object status, conditions and finalizers.
func (r *ProviderReconciler) patch(ctx context.Context, obj *apiv1.Provider, patcher *patch.SerialPatcher) (retErr error) {
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
