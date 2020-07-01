/*
Copyright 2020 The Flux CD contributors.

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
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/notification-controller/api/v1alpha1"
)

// ProviderReconciler reconciles a Provider object
type ProviderReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=notification.fluxcd.io,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.fluxcd.io,resources=providers/status,verbs=get;update;patch

func (r *ProviderReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	var provider v1alpha1.Provider
	if err := r.Get(ctx, req.NamespacedName, &provider); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues(strings.ToLower(provider.Kind), req.NamespacedName)

	init := true
	for _, condition := range provider.Status.Conditions {
		if condition.Type == v1alpha1.ReadyCondition && condition.Status == corev1.ConditionTrue {
			init = false
			break
		}
	}

	if init {
		provider.Status.Conditions = []v1alpha1.Condition{
			{
				Type:               v1alpha1.ReadyCondition,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             v1alpha1.InitializedReason,
				Message:            v1alpha1.InitializedReason,
			},
		}
		if err := r.Status().Update(ctx, &provider); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		log.Info("Provider initialised")
	}

	return ctrl.Result{}, nil
}

func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Provider{}).
		Complete(r)
}
