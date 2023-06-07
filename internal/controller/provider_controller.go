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
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
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

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/notification-controller/internal/notifier"
)

// insecureHTTPError occurs when insecure HTTP communication is tried
// and such behaviour is blocked.
var insecureHTTPError = errors.New("use of insecure plain HTTP connections is blocked")

// ProviderReconciler reconciles a Provider object
type ProviderReconciler struct {
	client.Client
	helper.Metrics
	kuberecorder.EventRecorder

	ControllerName    string
	BlockInsecureHTTP bool
}

type ProviderReconcilerOptions struct {
	RateLimiter ratelimiter.RateLimiter
}

func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, ProviderReconcilerOptions{})
}

func (r *ProviderReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts ProviderReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1beta2.Provider{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{}),
		)).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).
		Complete(r)
}

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *ProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	reconcileStart := time.Now()
	log := ctrl.LoggerFrom(ctx)

	obj := &apiv1beta2.Provider{}
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

		// Log the staleness error and pause reconciliation until spec changes.
		if conditions.IsStalled(obj) {
			result = ctrl.Result{Requeue: false}
			log.Error(retErr, "Reconciliation has stalled")
			retErr = nil
			return
		}

		// Log and emit success event.
		if retErr == nil && conditions.IsReady(obj) {
			msg := fmt.Sprintf("Reconciliation finished, next run in %s",
				obj.GetInterval().String())
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

func (r *ProviderReconciler) reconcile(ctx context.Context, obj *apiv1beta2.Provider) (ctrl.Result, error) {
	// Mark the resource as under reconciliation.
	conditions.MarkReconciling(obj, meta.ProgressingReason, "Reconciliation in progress")
	conditions.Delete(obj, meta.StalledCondition)

	// Mark the reconciliation as stalled if the inline URL and/or proxy are invalid.
	if err := r.validateURLs(obj); err != nil {
		var reason string
		if errors.Is(err, insecureHTTPError) {
			reason = meta.InsecureConnectionsDisallowedReason
		} else {
			reason = meta.InvalidURLReason
		}
		conditions.MarkFalse(obj, meta.ReadyCondition, reason, err.Error())
		conditions.MarkTrue(obj, meta.StalledCondition, reason, err.Error())
		return ctrl.Result{Requeue: true}, err
	}

	// Validate the provider credentials.
	if err := r.validateCredentials(ctx, obj); err != nil {
		var reason string
		var urlErr *url.Error
		if errors.Is(err, insecureHTTPError) {
			reason = meta.InsecureConnectionsDisallowedReason
		} else if errors.As(err, &urlErr) {
			reason = meta.InvalidURLReason
		} else {
			reason = apiv1.ValidationFailedReason
		}
		conditions.MarkFalse(obj, meta.ReadyCondition, reason, err.Error())
		return ctrl.Result{Requeue: true}, err
	}

	conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, apiv1.InitializedReason)
	return ctrl.Result{RequeueAfter: obj.GetInterval()}, nil
}

func (r *ProviderReconciler) validateURLs(provider *apiv1beta2.Provider) error {
	address := provider.Spec.Address
	proxy := provider.Spec.Proxy

	if provider.Spec.SecretRef == nil {
		return parseURLs(address, proxy, r.BlockInsecureHTTP)
	}
	return nil
}

func (r *ProviderReconciler) validateCredentials(ctx context.Context, provider *apiv1beta2.Provider) error {
	address := provider.Spec.Address
	proxy := provider.Spec.Proxy
	username := provider.Spec.Username
	password := ""
	token := ""
	headers := make(map[string]string)
	if provider.Spec.SecretRef != nil {
		// since a secret ref is provided, the object is not stalled even if spec.address
		// or spec.proxy are invalid, as the secret can change any time independently.
		if conditions.IsStalled(provider) {
			conditions.Delete(provider, meta.StalledCondition)
		}
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

	if err := parseURLs(address, proxy, r.BlockInsecureHTTP); err != nil {
		return err
	}

	factory := notifier.NewFactory(address, proxy, username, provider.Spec.Channel, token, headers, certPool, password, string(provider.UID))
	if _, err := factory.Notifier(provider.Spec.Type); err != nil {
		return fmt.Errorf("failed to initialize provider, error: %w", err)
	}

	return nil
}

// patch updates the object status, conditions and finalizers.
func (r *ProviderReconciler) patch(ctx context.Context, obj *apiv1beta2.Provider, patcher *patch.SerialPatcher) (retErr error) {
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

	// Remove the Reconciling/Stalled condition and update the observed generation
	// if the reconciliation was successful.
	if conditions.IsTrue(obj, meta.ReadyCondition) {
		conditions.Delete(obj, meta.ReconcilingCondition)
		conditions.Delete(obj, meta.StalledCondition)
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

	// Remove the Reconciling condition if the reconciliation has stalled.
	if conditions.Has(obj, meta.StalledCondition) {
		conditions.Delete(obj, meta.ReconcilingCondition)
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

// parseURLs parses the provided URL strings and returns any error that
// might occur when doing so. It raises an `insecureHTTPError` error when the
// scheme of either URL is "http" and `blockHTTP` is set to true.
func parseURLs(address, proxy string, blockHTTP bool) error {
	addrURL, err := url.ParseRequestURI(address)
	if err != nil {
		return fmt.Errorf("invalid address %s: %w", address, err)
	}
	proxyURL, err := url.ParseRequestURI(proxy)
	if proxy != "" && err != nil {
		return fmt.Errorf("invalid proxy %s: %w", proxy, err)
	}

	if proxyURL != nil && proxyURL.Scheme == "http" && blockHTTP {
		return fmt.Errorf("consider changing proxy to use HTTPS: %w", insecureHTTPError)
	}
	if addrURL != nil && addrURL.Scheme == "http" && blockHTTP {
		return fmt.Errorf("consider changing address to use HTTPS: %w", insecureHTTPError)
	}
	return nil
}
