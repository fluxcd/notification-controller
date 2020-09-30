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

	"github.com/fluxcd/pkg/apis/meta"

	"github.com/fluxcd/notification-controller/api/v1beta1"
)

// AlertReconciler reconciles a Alert object
type AlertReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=alerts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=alerts/status,verbs=get;update;patch

func (r *AlertReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	var alert v1beta1.Alert
	if err := r.Get(ctx, req.NamespacedName, &alert); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues("controller", strings.ToLower(alert.Kind), "request", req.NamespacedName)

	init := true
	if c := meta.GetCondition(alert.Status.Conditions, meta.ReadyCondition); c != nil {
		if c.Status == corev1.ConditionTrue {
			init = false
		}
	}

	if init {
		alert.Status.Conditions = []meta.Condition{
			{
				Type:               meta.ReadyCondition,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             v1beta1.InitializedReason,
				Message:            v1beta1.InitializedReason,
			},
		}
		if err := r.Status().Update(ctx, &alert); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		log.Info("Alert initialised")
	}

	return ctrl.Result{}, nil
}

func (r *AlertReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Alert{}).
		Complete(r)
}
