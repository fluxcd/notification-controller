package controllers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/ssa"

	notifyv1 "github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/internal/server"
)

func TestReceiverHandler(t *testing.T) {
	g := NewWithT(t)

	receiverServer := server.NewReceiverServer("127.0.0.1:56788", logf.Log, k8sClient)
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

	receiver := &notifyv1.Receiver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      receiverKey.Name,
			Namespace: receiverKey.Namespace,
		},
		Spec: notifyv1.ReceiverSpec{
			Type:   "generic",
			Events: []string{"pull"},
			Resources: []notifyv1.CrossNamespaceObjectReference{
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

	var rcvrObj notifyv1.Receiver
	g.Eventually(func() bool {
		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), &rcvrObj))
		return rcvrObj.Status.URL == address
	}, 30*time.Second, time.Second).Should(BeTrue())

	// Update receiver and check that url doesn't change
	rcvrObj.Spec.Events = []string{"ping", "push"}
	g.Expect(k8sClient.Update(context.Background(), &rcvrObj)).To(Succeed())
	g.Consistently(func() bool {
		var obj notifyv1.Receiver
		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(receiver), &obj)).To(Succeed())
		return obj.Status.URL == address
	}, 30*time.Second, time.Second).Should(BeTrue())

	res, err := http.Post("http://localhost:56788/"+address, "application/json", nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.StatusCode).To(Equal(http.StatusOK))
	g.Eventually(func() bool {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(object.GroupVersionKind())
		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(object), obj)).To(Succeed())
		v, ok := obj.GetAnnotations()[meta.ReconcileRequestAnnotation]
		return ok && v != ""
	}, 30*time.Second, time.Second).Should(BeTrue())
}

func readManifest(manifest, namespace string) (*unstructured.Unstructured, error) {
	data, err := os.ReadFile(manifest)
	if err != nil {
		return nil, err
	}
	yml := fmt.Sprintf(string(data), namespace)

	object, err := ssa.ReadObject(strings.NewReader(yml))
	if err != nil {
		return nil, err
	}

	return object, nil
}
