/*
Copyright 2024 The Flux authors

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

package server

import (
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	apimeta "github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/apis/meta"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

// newTestReceiverServer creates a ReceiverServer listening on a free port and
// returns the server, its base URL and a stop channel.  The caller must close
// the stop channel to shut the server down.
func newTestReceiverServer(t *testing.T, exportHTTPPathMetrics bool, objs ...runtime.Object) (*ReceiverServer, string, chan struct{}) {
	t.Helper()
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(apiv1.AddToScheme(scheme)).To(Succeed())

	builder := fakeclient.NewClientBuilder().WithScheme(scheme).
		WithIndex(&apiv1.Receiver{}, WebhookPathIndexKey, IndexReceiverWebhookPath)
	for _, o := range objs {
		builder = builder.WithRuntimeObjects(o)
	}
	kc := builder.Build()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	g.Expect(err).ToNot(HaveOccurred())
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	g.Expect(l.Close()).ToNot(HaveOccurred())

	// Use a fresh private Prometheus registry per test to avoid duplicate
	// metric registration panics when subtests each create a new server.
	mdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix:   "gotk_receiver_test",
			Registry: prometheus.NewRegistry(),
		}),
	})

	srv := NewReceiverServer(
		"127.0.0.1:"+port,
		log.Log,
		kc,
		false,
		exportHTTPPathMetrics,
	)
	stopCh := make(chan struct{})
	go srv.ListenAndServe(stopCh, mdlw)

	// Wait until the server is ready.
	baseURL := "http://127.0.0.1:" + port
	g.Eventually(func() error {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

	return srv, baseURL, stopCh
}

func TestNewReceiverServer(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(apiv1.AddToScheme(scheme)).To(Succeed())

	kc := fakeclient.NewClientBuilder().WithScheme(scheme).Build()

	srv := NewReceiverServer(":9292", log.Log, kc, true, true)
	g.Expect(srv).ToNot(BeNil())
	g.Expect(srv.port).To(Equal(":9292"))
	g.Expect(srv.kubeClient).ToNot(BeNil())
	g.Expect(srv.noCrossNamespaceRefs).To(BeTrue())
	g.Expect(srv.exportHTTPPathMetrics).To(BeTrue())
}

func TestReceiverServer_ListenAndServe(t *testing.T) {
	tests := []struct {
		name                  string
		path                  string
		exportHTTPPathMetrics bool
		wantStatus            int
	}{
		{
			name:       "unknown hook path returns 404",
			path:       apiv1.ReceiverWebhookPath + "unknowntoken",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "non-hook path returns 404",
			path:       "/healthz",
			wantStatus: http.StatusNotFound,
		},
		{
			name:                  "unknown hook path with exportHTTPPathMetrics returns 404",
			path:                  apiv1.ReceiverWebhookPath + "unknowntoken",
			exportHTTPPathMetrics: true,
			wantStatus:            http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			_, baseURL, stopCh := newTestReceiverServer(t, tt.exportHTTPPathMetrics)
			defer close(stopCh)

			resp, err := http.Post(baseURL+tt.path, "application/json", nil)
			g.Expect(err).ToNot(HaveOccurred())
			resp.Body.Close()
			g.Expect(resp.StatusCode).To(Equal(tt.wantStatus))
		})
	}
}

func TestReceiverServer_Shutdown(t *testing.T) {
	g := NewWithT(t)

	_, baseURL, stopCh := newTestReceiverServer(t, false)

	// Server is up.
	resp, err := http.Get(baseURL + "/")
	g.Expect(err).ToNot(HaveOccurred())
	resp.Body.Close()

	// Signal shutdown.
	close(stopCh)

	// Server should stop accepting connections.
	g.Eventually(func() error {
		_, err := http.Get(baseURL + "/")
		return err
	}, 5*time.Second, 100*time.Millisecond).ShouldNot(Succeed())
}

func TestReceiverServer_WebhookPathRouting(t *testing.T) {
	g := NewWithT(t)

	// Create a Receiver with a known token so handlePayload can look it up.
	token := "test-token-abc123"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "receiver-token",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte(token),
		},
	}
	receiver := &apiv1.Receiver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-receiver",
			Namespace: "default",
		},
		Spec: apiv1.ReceiverSpec{
			Type: apiv1.GenericReceiver,
			SecretRef: meta.LocalObjectReference{
				Name: "receiver-token",
			},
		},
		Status: apiv1.ReceiverStatus{
			WebhookPath: apiv1.ReceiverWebhookPath + token,
			Conditions: []metav1.Condition{
				{
					Type:   apimeta.ReadyCondition,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}

	_, baseURL, stopCh := newTestReceiverServer(t, false, secret, receiver)
	defer close(stopCh)

	// A POST to the webhook path with a valid token should reach handlePayload
	// and return 200 (GenericReceiver with Ready=True).
	resp, err := http.Post(baseURL+apiv1.ReceiverWebhookPath+token, "application/json", nil)
	g.Expect(err).ToNot(HaveOccurred())
	resp.Body.Close()
	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
}
