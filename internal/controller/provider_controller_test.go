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
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
)

func TestProviderReconciler_Reconcile(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second
	resultP := &apiv1beta2.Provider{}
	namespaceName := "provider-" + randStringRunes(5)
	secretName := "secret-" + randStringRunes(5)

	g.Expect(createNamespace(namespaceName)).NotTo(HaveOccurred(), "failed to create test namespace")

	providerKey := types.NamespacedName{
		Name:      fmt.Sprintf("provider-%s", randStringRunes(5)),
		Namespace: namespaceName,
	}
	provider := &apiv1beta2.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerKey.Name,
			Namespace: providerKey.Namespace,
		},
		Spec: apiv1beta2.ProviderSpec{
			Type:    "generic",
			Address: "https://webhook.internal",
		},
	}
	g.Expect(k8sClient.Create(context.Background(), provider)).To(Succeed())

	t.Run("reports ready status", func(t *testing.T) {
		g := NewWithT(t)

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)
			return resultP.Status.ObservedGeneration == resultP.Generation
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.IsReady(resultP)).To(BeTrue())
		g.Expect(conditions.GetReason(resultP, meta.ReadyCondition)).To(BeIdenticalTo(meta.SucceededReason))

		g.Expect(conditions.Has(resultP, meta.ReconcilingCondition)).To(BeFalse())
		g.Expect(controllerutil.ContainsFinalizer(resultP, apiv1.NotificationFinalizer)).To(BeTrue())
	})

	t.Run("fails with secret not found error", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)).To(Succeed())

		resultP.Spec.SecretRef = &meta.LocalObjectReference{
			Name: secretName,
		}
		g.Expect(k8sClient.Update(context.Background(), resultP)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)
			return !conditions.IsReady(resultP)
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.GetReason(resultP, meta.ReadyCondition)).To(BeIdenticalTo(apiv1.ValidationFailedReason))
		g.Expect(conditions.GetMessage(resultP, meta.ReadyCondition)).To(ContainSubstring(secretName))

		g.Expect(conditions.Has(resultP, meta.ReconcilingCondition)).To(BeTrue())
		g.Expect(conditions.GetReason(resultP, meta.ReconcilingCondition)).To(BeIdenticalTo(meta.ProgressingWithRetryReason))
		g.Expect(conditions.GetObservedGeneration(resultP, meta.ReconcilingCondition)).To(BeIdenticalTo(resultP.Generation))
		g.Expect(resultP.Status.ObservedGeneration).To(BeIdenticalTo(resultP.Generation - 1))
	})

	t.Run("recovers when secret exists", func(t *testing.T) {
		g := NewWithT(t)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespaceName,
			},
			StringData: map[string]string{
				"token": "test",
			},
		}
		g.Expect(k8sClient.Create(context.Background(), secret)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)
			return conditions.IsReady(resultP)
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.GetObservedGeneration(resultP, meta.ReadyCondition)).To(BeIdenticalTo(resultP.Generation))
		g.Expect(resultP.Status.ObservedGeneration).To(BeIdenticalTo(resultP.Generation))
		g.Expect(conditions.Has(resultP, meta.ReconcilingCondition)).To(BeFalse())
	})

	t.Run("handles reconcileAt", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)).To(Succeed())

		reconcileRequestAt := metav1.Now().String()
		resultP.SetAnnotations(map[string]string{
			meta.ReconcileRequestAnnotation: reconcileRequestAt,
		})
		g.Expect(k8sClient.Update(context.Background(), resultP)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)
			return resultP.Status.LastHandledReconcileAt == reconcileRequestAt
		}, timeout, time.Second).Should(BeTrue())
	})

	t.Run("becomes stalled on invalid proxy", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)).To(Succeed())

		resultP.Spec.SecretRef = nil
		resultP.Spec.Proxy = "https://proxy.internal|"
		g.Expect(k8sClient.Update(context.Background(), resultP)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)
			return !conditions.IsReady(resultP)
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.Has(resultP, meta.ReconcilingCondition)).To(BeFalse())
		g.Expect(conditions.Has(resultP, meta.StalledCondition)).To(BeTrue())
		g.Expect(conditions.GetObservedGeneration(resultP, meta.StalledCondition)).To(BeIdenticalTo(resultP.Generation))
		g.Expect(conditions.GetReason(resultP, meta.StalledCondition)).To(BeIdenticalTo(meta.InvalidURLReason))
		g.Expect(conditions.GetReason(resultP, meta.ReadyCondition)).To(BeIdenticalTo(meta.InvalidURLReason))
	})

	t.Run("recovers from staleness", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)).To(Succeed())

		resultP.Spec.Proxy = "https://proxy.internal"
		g.Expect(k8sClient.Update(context.Background(), resultP)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)
			return conditions.IsReady(resultP)
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.Has(resultP, meta.ReconcilingCondition)).To(BeFalse())
		g.Expect(conditions.Has(resultP, meta.StalledCondition)).To(BeFalse())
	})

	t.Run("finalizes suspended object", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)).To(Succeed())

		patchHelper, err := patch.NewHelper(resultP, k8sClient)
		g.Expect(err).ToNot(HaveOccurred())
		resultP.Spec.Suspend = true
		g.Expect(patchHelper.Patch(context.Background(), resultP)).ToNot(HaveOccurred())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)
			return resultP.Spec.Suspend == true
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(k8sClient.Delete(context.Background(), resultP)).To(Succeed())

		g.Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), resultP)
			return apierrors.IsNotFound(err)
		}, timeout, time.Second).Should(BeTrue())
	})
}
