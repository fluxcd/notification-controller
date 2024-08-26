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
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/ssa"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	"github.com/fluxcd/notification-controller/internal/server"
)

func TestReceiverReconciler_deleteBeforeFinalizer(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "receiver-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	receiver := &apiv1.Receiver{}
	receiver.Name = "test-receiver"
	receiver.Namespace = namespaceName
	receiver.Spec = apiv1.ReceiverSpec{
		Type: "github",
		Resources: []apiv1.CrossNamespaceObjectReference{
			{Kind: "Bucket", Name: "Foo"},
		},
		SecretRef: meta.LocalObjectReference{Name: "foo-secret"},
	}
	// Add a test finalizer to prevent the object from getting deleted.
	receiver.SetFinalizers([]string{"test-finalizer"})
	g.Expect(k8sClient.Create(ctx, receiver)).NotTo(HaveOccurred())
	// Add deletion timestamp by deleting the object.
	g.Expect(k8sClient.Delete(ctx, receiver)).NotTo(HaveOccurred())

	r := &ReceiverReconciler{
		Client:        k8sClient,
		EventRecorder: record.NewFakeRecorder(32),
	}
	// NOTE: Only a real API server responds with an error in this scenario.
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(receiver)})
	g.Expect(err).NotTo(HaveOccurred())
}

func TestReceiverReconciler_Reconcile(t *testing.T) {
	g := NewWithT(t)

	timeout := 5 * time.Second
	resultR := &apiv1.Receiver{}
	namespaceName := "receiver-" + randStringRunes(5)
	secretName := "secret-" + randStringRunes(5)

	g.Expect(createNamespace(namespaceName)).NotTo(HaveOccurred(), "failed to create test namespace")

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

	receiverKey := types.NamespacedName{
		Name:      fmt.Sprintf("receiver-%s", randStringRunes(5)),
		Namespace: namespaceName,
	}
	receiver := &apiv1.Receiver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      receiverKey.Name,
			Namespace: receiverKey.Namespace,
		},
		Spec: apiv1.ReceiverSpec{
			Type:   "generic",
			Events: []string{"push"},
			Resources: []apiv1.CrossNamespaceObjectReference{
				{
					Name: "podinfo",
					Kind: "GitRepository",
				},
			},
			SecretRef: meta.LocalObjectReference{
				Name: secretName,
			},
		},
	}
	g.Expect(k8sClient.Create(context.Background(), receiver)).To(Succeed())

	t.Run("reports ready status", func(t *testing.T) {
		g := NewWithT(t)

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)
			return resultR.Status.ObservedGeneration == resultR.Generation
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.IsReady(resultR)).To(BeTrue())
		g.Expect(conditions.GetReason(resultR, meta.ReadyCondition)).To(BeIdenticalTo(meta.SucceededReason))

		g.Expect(conditions.Has(resultR, meta.ReconcilingCondition)).To(BeFalse())
		g.Expect(controllerutil.ContainsFinalizer(resultR, apiv1.NotificationFinalizer)).To(BeTrue())
		g.Expect(resultR.Spec.Interval.Duration).To(BeIdenticalTo(10 * time.Minute))
	})

	t.Run("fails with secret not found error", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Delete(context.Background(), secret)).To(Succeed())

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)).To(Succeed())

		reconcileRequestAt := metav1.Now().String()
		resultR.SetAnnotations(map[string]string{
			meta.ReconcileRequestAnnotation: reconcileRequestAt,
		})
		g.Expect(k8sClient.Update(context.Background(), resultR)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)
			return !conditions.IsReady(resultR)
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.GetReason(resultR, meta.ReadyCondition)).To(BeIdenticalTo(apiv1.TokenNotFoundReason))
		g.Expect(conditions.GetMessage(resultR, meta.ReadyCondition)).To(ContainSubstring(secretName))

		g.Expect(conditions.Has(resultR, meta.ReconcilingCondition)).To(BeTrue())
		g.Expect(conditions.GetReason(resultR, meta.ReconcilingCondition)).To(BeIdenticalTo(meta.ProgressingWithRetryReason))
		g.Expect(conditions.GetObservedGeneration(resultR, meta.ReconcilingCondition)).To(BeIdenticalTo(resultR.Generation))
	})

	t.Run("recovers when secret exists", func(t *testing.T) {
		g := NewWithT(t)
		newSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespaceName,
			},
			StringData: map[string]string{
				"token": "test",
			},
		}
		g.Expect(k8sClient.Create(context.Background(), newSecret)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)
			return conditions.IsReady(resultR)
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.GetObservedGeneration(resultR, meta.ReadyCondition)).To(BeIdenticalTo(resultR.Generation))
		g.Expect(resultR.Status.ObservedGeneration).To(BeIdenticalTo(resultR.Generation))
		g.Expect(conditions.Has(resultR, meta.ReconcilingCondition)).To(BeFalse())
	})

	t.Run("handles reconcileAt", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)).To(Succeed())

		reconcileRequestAt := metav1.Now().String()
		resultR.SetAnnotations(map[string]string{
			meta.ReconcileRequestAnnotation: reconcileRequestAt,
		})
		g.Expect(k8sClient.Update(context.Background(), resultR)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)
			return resultR.Status.LastHandledReconcileAt == reconcileRequestAt
		}, timeout, time.Second).Should(BeTrue())
	})

	t.Run("finalizes suspended object", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)).To(Succeed())

		resultR.Spec.Suspend = true
		g.Expect(k8sClient.Update(context.Background(), resultR)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)
			return resultR.Spec.Suspend == true
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(k8sClient.Delete(context.Background(), resultR)).To(Succeed())

		g.Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)
			return apierrors.IsNotFound(err)
		}, timeout, time.Second).Should(BeTrue())
	})
}

func TestReceiverReconciler_EventHandler(t *testing.T) {
	g := NewWithT(t)
	timeout := 30 * time.Second
	resultR := &apiv1.Receiver{}

	// Use the client from the manager as the server handler needs to list objects from the cache
	// which the "live" k8s client does not have access to.
	receiverServer := server.NewReceiverServer("127.0.0.1:56788", logf.Log, testEnv.GetClient(), true)
	receiverMdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix: "gotk_receiver",
		}),
	})
	stopCh := make(chan struct{})
	go receiverServer.ListenAndServe(stopCh, receiverMdlw)
	defer close(stopCh)

	id := "rcvr-" + randStringRunes(5)
	err := createNamespace(id)
	g.Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")

	object, err := readManifest("./testdata/repo.yaml", id)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = manager.Apply(context.Background(), object, ssa.ApplyOptions{
		Force: true,
	})
	g.Expect(err).ToNot(HaveOccurred())

	token := "test-token"
	secretKey := types.NamespacedName{
		Namespace: id,
		Name:      "receiver-secret",
	}

	receiverSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretKey.Name,
			Namespace: secretKey.Namespace,
		},
		StringData: map[string]string{
			"token": token,
		},
	}
	g.Expect(k8sClient.Create(context.Background(), receiverSecret))

	receiverKey := types.NamespacedName{
		Namespace: id,
		Name:      fmt.Sprintf("test-receiver-%s", randStringRunes(5)),
	}

	receiver := &apiv1.Receiver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      receiverKey.Name,
			Namespace: receiverKey.Namespace,
		},
		Spec: apiv1.ReceiverSpec{
			Type:   "generic",
			Events: []string{"pull"},
			Resources: []apiv1.CrossNamespaceObjectReference{
				{
					Name: "podinfo",
					Kind: "GitRepository",
				},
			},
			SecretRef: meta.LocalObjectReference{
				Name: "receiver-secret",
			},
		},
	}

	g.Expect(k8sClient.Create(context.Background(), receiver)).To(Succeed())

	address := fmt.Sprintf("/hook/%s", sha256sum(token+receiverKey.Name+receiverKey.Namespace))

	t.Run("generates URL when ready", func(t *testing.T) {
		g := NewWithT(t)
		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)
			return conditions.IsReady(resultR)
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(resultR.Status.WebhookPath).To(BeIdenticalTo(address))
		g.Expect(conditions.GetMessage(resultR, meta.ReadyCondition)).To(ContainSubstring(address))
	})

	t.Run("doesn't update the URL on spec updates", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)).To(Succeed())

		resultR.Spec.Events = []string{"ping", "push"}
		g.Expect(k8sClient.Update(context.Background(), resultR)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), resultR)
			return resultR.Status.ObservedGeneration == resultR.Generation
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.IsReady(resultR))
		g.Expect(resultR.Status.WebhookPath).To(BeIdenticalTo(address))
	})

	t.Run("handles event", func(t *testing.T) {
		g := NewWithT(t)
		res, err := http.Post("http://localhost:56788/"+address, "application/json", nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res.StatusCode).To(Equal(http.StatusOK))

		g.Eventually(func() bool {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(object.GroupVersionKind())
			g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(object), obj)).To(Succeed())
			v, ok := obj.GetAnnotations()[meta.ReconcileRequestAnnotation]
			return ok && v != ""
		}, timeout, time.Second).Should(BeTrue())
	})
}
