/*
Copyright 2026 The Flux authors

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
	"testing"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	runtimectrl "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/metrics"
)

func durationSeriesExists(t *testing.T, rec *metrics.Recorder, kind, name, namespace string) bool {
	t.Helper()

	var hv *prometheus.HistogramVec
	for _, c := range rec.Collectors() {
		if v, ok := c.(*prometheus.HistogramVec); ok {
			hv = v
			break
		}
	}
	if hv == nil {
		t.Fatal("no *prometheus.HistogramVec collector found on the recorder")
	}

	ch := make(chan prometheus.Metric, 64)
	hv.Collect(ch)
	close(ch)

	for m := range ch {
		var pb dto.Metric
		if err := m.Write(&pb); err != nil {
			t.Fatalf("failed to write metric: %v", err)
		}
		labels := make(map[string]string, len(pb.GetLabel()))
		for _, lp := range pb.GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}
		if labels["kind"] == kind && labels["name"] == name && labels["namespace"] == namespace {
			return true
		}
	}
	return false
}

func TestReceiverReconciler_deleteMetricsOnNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	const namespaceName = "receiver-metrics-test"
	const name = "metrics-test"

	obj := &apiv1.Receiver{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespaceName,
			Finalizers: []string{apiv1.NotificationFinalizer},
		},
		Spec: apiv1.ReceiverSpec{
			Type:      "generic",
			SecretRef: &meta.LocalObjectReference{Name: "token"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithStatusSubresource(&apiv1.Receiver{}).
		WithObjects(obj).
		Build()

	rec := metrics.NewRecorder()

	r := &ReceiverReconciler{
		Client:   fakeClient,
		Recorder: events.NewFakeRecorder(32),
		Metrics: runtimectrl.Metrics{
			Scheme:          scheme.Scheme,
			MetricsRecorder: rec,
		},
	}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(obj)}

	// The secret reference doesn't resolve, so this reconcile returns an
	// error -- but Get still succeeded, so the deferred RecordDuration
	// already ran before that error was returned, creating a series.
	_, _ = r.Reconcile(ctx, req)
	g.Expect(durationSeriesExists(t, rec, apiv1.ReceiverKind, name, namespaceName)).
		To(BeTrue(), "expected a duration series after the first (successful Get) reconcile")

	g.Expect(fakeClient.Get(ctx, req.NamespacedName, obj)).To(Succeed())
	obj.SetFinalizers(nil)
	g.Expect(fakeClient.Update(ctx, obj)).To(Succeed())
	g.Expect(fakeClient.Delete(ctx, obj)).To(Succeed())

	err := fakeClient.Get(ctx, req.NamespacedName, &apiv1.Receiver{})
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "object must be fully gone (NotFound), not merely marked for deletion")

	_, err = r.Reconcile(ctx, req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(durationSeriesExists(t, rec, apiv1.ReceiverKind, name, namespaceName)).
		To(BeFalse(), "duration series must not survive the object's deletion")
}
