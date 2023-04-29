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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fluxcd/pkg/ssa"
	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-limiter/memorystore"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/notification-controller/internal/server"
)

func TestAlertReconciler_Reconcile(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second
	resultA := &apiv1beta2.Alert{}
	namespaceName := "alert-" + randStringRunes(5)
	providerName := "provider-" + randStringRunes(5)

	g.Expect(createNamespace(namespaceName)).NotTo(HaveOccurred(), "failed to create test namespace")

	provider := &apiv1beta2.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: namespaceName,
		},
		Spec: apiv1beta2.ProviderSpec{
			Type:    "generic",
			Address: "https://webhook.internal",
		},
	}

	alert := &apiv1beta2.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("alert-%s", randStringRunes(5)),
			Namespace: namespaceName,
		},
		Spec: apiv1beta2.AlertSpec{
			ProviderRef: meta.LocalObjectReference{
				Name: providerName,
			},
			EventSeverity: "info",
			EventSources: []apiv1.CrossNamespaceObjectReference{
				{
					Kind: "Bucket",
					Name: "*",
				},
			},
		},
	}
	g.Expect(k8sClient.Create(context.Background(), alert)).To(Succeed())

	t.Run("fails with provider not found error", func(t *testing.T) {
		g := NewWithT(t)

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), resultA)
			return conditions.Has(resultA, meta.ReadyCondition)
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.IsReady(resultA)).To(BeFalse())
		g.Expect(conditions.GetReason(resultA, meta.ReadyCondition)).To(BeIdenticalTo(meta.FailedReason))
		g.Expect(conditions.GetMessage(resultA, meta.ReadyCondition)).To(ContainSubstring(providerName))

		g.Expect(conditions.Has(resultA, meta.ReconcilingCondition)).To(BeTrue())
		g.Expect(conditions.GetReason(resultA, meta.ReconcilingCondition)).To(BeIdenticalTo(meta.ProgressingWithRetryReason))
		g.Expect(conditions.GetObservedGeneration(resultA, meta.ReconcilingCondition)).To(BeIdenticalTo(resultA.Generation))
		g.Expect(controllerutil.ContainsFinalizer(resultA, apiv1.NotificationFinalizer)).To(BeTrue())
	})

	t.Run("recovers when provider exists", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Create(context.Background(), provider)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), resultA)
			return conditions.IsReady(resultA)
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(conditions.GetObservedGeneration(resultA, meta.ReadyCondition)).To(BeIdenticalTo(resultA.Generation))
		g.Expect(resultA.Status.ObservedGeneration).To(BeIdenticalTo(resultA.Generation))
		g.Expect(conditions.Has(resultA, meta.ReconcilingCondition)).To(BeFalse())
	})

	t.Run("handles reconcileAt", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), resultA)).To(Succeed())

		reconcileRequestAt := metav1.Now().String()
		resultA.SetAnnotations(map[string]string{
			meta.ReconcileRequestAnnotation: reconcileRequestAt,
		})
		g.Expect(k8sClient.Update(context.Background(), resultA)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), resultA)
			return resultA.Status.LastHandledReconcileAt == reconcileRequestAt
		}, timeout, time.Second).Should(BeTrue())
	})

	t.Run("finalizes suspended object", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), resultA)).To(Succeed())

		resultA.Spec.Suspend = true
		g.Expect(k8sClient.Update(context.Background(), resultA)).To(Succeed())

		g.Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), resultA)
			return resultA.Spec.Suspend == true
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(k8sClient.Delete(context.Background(), resultA)).To(Succeed())

		g.Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), resultA)
			return apierrors.IsNotFound(err)
		}, timeout, time.Second).Should(BeTrue())
	})
}

func TestAlertReconciler_EventHandler(t *testing.T) {
	g := NewWithT(t)
	var (
		namespace = "events-" + randStringRunes(5)
		req       *http.Request
		provider  *apiv1beta2.Provider
	)
	g.Expect(createNamespace(namespace)).NotTo(HaveOccurred(), "failed to create test namespace")

	eventMdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix: "gotk_event",
		}),
	})

	store, err := memorystore.New(&memorystore.Config{
		Interval: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create memory storage")
	}

	eventServer := server.NewEventServer("127.0.0.1:56789", logf.Log, k8sClient, true)
	stopCh := make(chan struct{})
	go eventServer.ListenAndServe(stopCh, eventMdlw, store)

	rcvServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		w.WriteHeader(200)
	}))
	defer rcvServer.Close()
	defer close(stopCh)

	providerKey := types.NamespacedName{
		Name:      fmt.Sprintf("provider-%s", randStringRunes(5)),
		Namespace: namespace,
	}
	provider = &apiv1beta2.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerKey.Name,
			Namespace: providerKey.Namespace,
		},
		Spec: apiv1beta2.ProviderSpec{
			Type:    "generic",
			Address: rcvServer.URL,
		},
	}
	g.Expect(k8sClient.Create(context.Background(), provider)).To(Succeed())
	g.Eventually(func() bool {
		var obj apiv1beta2.Provider
		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(provider), &obj))
		return conditions.IsReady(&obj)
	}, 30*time.Second, time.Second).Should(BeTrue())

	repo, err := readManifest("./testdata/repo.yaml", namespace)
	g.Expect(err).ToNot(HaveOccurred())

	secondRepo, err := readManifest("./testdata/gitrepo2.yaml", namespace)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = manager.Apply(context.Background(), repo, ssa.ApplyOptions{
		Force: true,
	})
	g.Expect(err).ToNot(HaveOccurred())

	_, err = manager.Apply(context.Background(), secondRepo, ssa.ApplyOptions{
		Force: true,
	})
	g.Expect(err).ToNot(HaveOccurred())

	alertKey := types.NamespacedName{
		Name:      fmt.Sprintf("alert-%s", randStringRunes(5)),
		Namespace: namespace,
	}

	alert := &apiv1beta2.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertKey.Name,
			Namespace: alertKey.Namespace,
		},
		Spec: apiv1beta2.AlertSpec{
			ProviderRef: meta.LocalObjectReference{
				Name: providerKey.Name,
			},
			EventSeverity: "info",
			EventSources: []apiv1.CrossNamespaceObjectReference{
				{
					Kind:      "Bucket",
					Name:      "hyacinth",
					Namespace: namespace,
				},
				{
					Kind: "Kustomization",
					Name: "*",
				},
				{
					Kind: "GitRepository",
					Name: "*",
					MatchLabels: map[string]string{
						"app": "podinfo",
					},
				},
				{
					Kind:      "Kustomization",
					Name:      "*",
					Namespace: "test",
				},
			},
			ExclusionList: []string{
				"doesnotoccur", // not intended to match
				"excluded",
			},
		},
	}
	inclusionAlert := alert.DeepCopy()
	inclusionAlert.Spec.InclusionList = []string{"^included"}

	g.Expect(k8sClient.Create(context.Background(), alert)).To(Succeed())

	// wait for controller to mark the alert as ready
	g.Eventually(func() bool {
		var obj apiv1beta2.Alert
		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), &obj))
		return conditions.IsReady(&obj)
	}, 30*time.Second, time.Second).Should(BeTrue())

	event := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Bucket",
			Name:      "hyacinth",
			Namespace: namespace,
		},
		Severity:            "info",
		Timestamp:           metav1.Now(),
		Message:             "well that happened",
		Reason:              "event-happened",
		ReportingController: "source-controller",
	}

	testSent := func() {
		buf := &bytes.Buffer{}
		g.Expect(json.NewEncoder(buf).Encode(&event)).To(Succeed())
		res, err := http.Post("http://localhost:56789/", "application/json", buf)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res.StatusCode).To(Equal(202)) // event_server responds with 202 Accepted
	}

	testForwarded := func() {
		g.Eventually(func() bool {
			return req == nil
		}, "2s", "0.1s").Should(BeFalse())
	}

	testFiltered := func() {
		// The event_server does forwarding in a goroutine, after
		// responding to the POST of the event. This makes it
		// difficult to know whether the provider has filtered the
		// event, or just not run the goroutine yet. For now, I'll use
		// a timeout (and Consistently so it can fail early)
		g.Consistently(func() bool {
			return req == nil
		}, "1s", "0.1s").Should(BeTrue())
	}

	tests := []struct {
		name            string
		modifyEventFunc func(e eventv1.Event) eventv1.Event
		forwarded       bool
	}{
		{
			name:            "forwards when source is a match",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event { return e },
			forwarded:       true,
		},
		{
			name: "drops event when source Kind does not match",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.InvolvedObject.Kind = "GitRepository"
				return e
			},
			forwarded: false,
		},
		{
			name: "drops event when source name does not match",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.InvolvedObject.Name = "slop"
				return e
			},
			forwarded: false,
		},
		{
			name: "drops event when source namespace does not match",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.InvolvedObject.Namespace = "all-buckets"
				return e
			},
			forwarded: false,
		},
		{
			name: "drops event that is matched by exclusion",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.Message = "this is excluded"
				return e
			},
			forwarded: false,
		},
		{
			name: "forwards events when name wildcard is used",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.InvolvedObject.Kind = "Kustomization"
				e.InvolvedObject.Name = "test"
				e.InvolvedObject.Namespace = namespace
				e.Message = "test"
				return e
			},
			forwarded: true,
		},
		{
			name: "forwards events when the label matches",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.InvolvedObject.Kind = "GitRepository"
				e.InvolvedObject.Name = "podinfo"
				e.InvolvedObject.APIVersion = "source.toolkit.fluxcd.io/v1beta1"
				e.InvolvedObject.Namespace = namespace
				e.Message = "test"
				return e
			},
			forwarded: true,
		},
		{
			name: "drops events when the labels don't match",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.InvolvedObject.Kind = "GitRepository"
				e.InvolvedObject.Name = "podinfo-two"
				e.InvolvedObject.APIVersion = "source.toolkit.fluxcd.io/v1beta1"
				e.InvolvedObject.Namespace = namespace
				e.Message = "test"
				return e
			},
			forwarded: false,
		},
		{
			name: "drops events for cross-namespace sources",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.InvolvedObject.Kind = "Kustomization"
				e.InvolvedObject.Name = "test"
				e.InvolvedObject.Namespace = "test"
				e.Message = "test"
				return e
			},
			forwarded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event = tt.modifyEventFunc(event)
			testSent()
			if tt.forwarded {
				testForwarded()
			} else {
				testFiltered()
			}
			req = nil
		})
	}

	// update alert for testing inclusion list
	var obj apiv1beta2.Alert
	g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), &obj)).To(Succeed())
	inclusionAlert.ResourceVersion = obj.ResourceVersion
	g.Expect(k8sClient.Update(context.Background(), inclusionAlert)).To(Succeed())

	// wait for ready
	g.Eventually(func() bool {
		var obj apiv1beta2.Alert
		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(inclusionAlert), &obj))
		return conditions.IsReady(&obj)
	}, 30*time.Second, time.Second).Should(BeTrue())

	event = eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Bucket",
			Name:      "hyacinth",
			Namespace: namespace,
		},
		Severity:            "info",
		Timestamp:           metav1.Now(),
		Message:             "included",
		Reason:              "event-happened",
		ReportingController: "source-controller",
	}

	tests = []struct {
		name            string
		modifyEventFunc func(e eventv1.Event) eventv1.Event
		forwarded       bool
	}{
		{
			name:            "forwards when message matches inclusion list",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event { return e },
			forwarded:       true,
		},
		{
			name: "drops when message does not match inclusion list",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.Message = "not included"
				return e
			},
			forwarded: false,
		},
		{
			name: "drops when message matches inclusion list and exclusion list",
			modifyEventFunc: func(e eventv1.Event) eventv1.Event {
				e.Message = "included excluded"
				return e
			},
			forwarded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event = tt.modifyEventFunc(event)
			testSent()
			if tt.forwarded {
				testForwarded()
			} else {
				testFiltered()
			}
			req = nil
		})
	}
}
