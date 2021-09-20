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
	"crypto/x509"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/predicates"

	"github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/internal/notifier"
)

// ProviderReconciler reconciles a Provider object
type ProviderReconciler struct {
	client.Client
	helper.Metrics

	Scheme *runtime.Scheme
}

type ProviderReconcilerOptions struct {
	MaxConcurrentReconciles int
}

func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, ProviderReconcilerOptions{})
}

func (r *ProviderReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts ProviderReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Provider{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{})).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers/status,verbs=get;update;patch

func (r *ProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	start := time.Now()
	log := ctrl.LoggerFrom(ctx)

	provider := &v1beta1.Provider{}
	if err := r.Get(ctx, req.NamespacedName, provider); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.RecordSuspend(ctx, provider, provider.Spec.Suspend)
	// return early if the object is suspended
	if provider.Spec.Suspend {
		log.Info("Reconciliation is suspended for this object")
		return ctrl.Result{}, nil
	}

	patchHelper, err := patch.NewHelper(provider, r.Client)
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
			conditions.Delete(provider, meta.ReconcilingCondition)

			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})

			readyCondition := conditions.Get(provider, meta.ReadyCondition)
			switch readyCondition.Status {
			case metav1.ConditionFalse:
				// As we are no longer reconciling and the end-state is not ready, the reconciliation has stalled
				conditions.MarkStalled(provider, readyCondition.Reason, readyCondition.Message)
			case metav1.ConditionTrue:
				// As we are no longer reconciling and the end-state is ready, the reconciliation is no longer stalled
				conditions.Delete(provider, meta.StalledCondition)
			}
		}

		if err := patchHelper.Patch(ctx, provider, patchOpts...); err != nil {
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}

		r.Metrics.RecordReadiness(ctx, provider)
		r.Metrics.RecordDuration(ctx, provider, start)

	}()

	return r.reconcile(ctx, provider)
}

func (r *ProviderReconciler) reconcile(ctx context.Context, obj *v1beta1.Provider) (ctrl.Result, error) {
	// Mark the resource as under reconciliation
	conditions.MarkReconciling(obj, meta.ProgressingReason, "")

	// validate provider spec and credentials
	if err := r.validate(ctx, obj); err != nil {
		conditions.MarkFalse(obj, meta.ReadyCondition, meta.FailedReason, err.Error())
		return ctrl.Result{}, err
	}

	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, v1beta1.InitializedReason)
	ctrl.LoggerFrom(ctx).Info("Provider initialized")

	return ctrl.Result{}, nil
}

func (r *ProviderReconciler) validate(ctx context.Context, provider *v1beta1.Provider) error {
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

	factory := notifier.NewFactory(address, provider.Spec.Proxy, provider.Spec.Username, provider.Spec.Channel, token, certPool)
	if _, err := factory.Notifier(provider.Spec.Type); err != nil {
		return fmt.Errorf("failed to initialize provider, error: %w", err)
	}

	return nil
}
