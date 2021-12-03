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

	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-limiter/memorystore"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/events"

	notifyv1 "github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/internal/server"
)

func TestEventHandler(t *testing.T) {
	// randomize var? create http server here?
	g := NewWithT(t)
	var (
		namespace = "events-" + randStringRunes(5)
		req       *http.Request
		provider  *notifyv1.Provider
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

	eventServer := server.NewEventServer("127.0.0.1:56789", logf.Log, k8sClient)
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
	provider = &notifyv1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerKey.Name,
			Namespace: providerKey.Namespace,
		},
		Spec: notifyv1.ProviderSpec{
			Type:    "generic",
			Address: rcvServer.URL,
		},
	}
	g.Expect(k8sClient.Create(context.Background(), provider)).To(Succeed())

	alertKey := types.NamespacedName{
		Name:      fmt.Sprintf("alert-%s", randStringRunes(5)),
		Namespace: namespace,
	}

	alert := &notifyv1.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertKey.Name,
			Namespace: alertKey.Namespace,
		},
		Spec: notifyv1.AlertSpec{
			ProviderRef: meta.LocalObjectReference{
				Name: providerKey.Name,
			},
			EventSeverity: "info",
			EventSources: []notifyv1.CrossNamespaceObjectReference{
				{
					Kind:      "Bucket",
					Name:      "hyacinth",
					Namespace: namespace,
				},
			},
			ExclusionList: []string{
				"doesnotoccur", // not intended to match
				"excluded",
			},
		},
	}

	g.Expect(k8sClient.Create(context.Background(), alert)).To(Succeed())

	// wait for controller to mark the alert as ready
	g.Eventually(func() bool {
		var obj notifyv1.Alert
		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(alert), &obj))
		return conditions.IsReady(&obj)
	}, 30*time.Second, time.Second).Should(BeTrue())

	event := events.Event{
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

	t.Run("event forwarding", func(t *testing.T) {
		tests := []struct {
			name            string
			modifyEventFunc func(e events.Event) events.Event
			forwarded       bool
		}{
			{
				name:            "forwards when source is a match",
				modifyEventFunc: func(e events.Event) events.Event { return e },
				forwarded:       true,
			},
			{
				name: "drops event when source Kind does not match",
				modifyEventFunc: func(e events.Event) events.Event {
					e.InvolvedObject.Kind = "GitRepository"
					return e
				},
				forwarded: false,
			},
			{
				name: "drops event when source name does not match",
				modifyEventFunc: func(e events.Event) events.Event {
					e.InvolvedObject.Name = "slop"
					return e
				},
				forwarded: false,
			},
			{
				name: "drops event when source namespace does not match",
				modifyEventFunc: func(e events.Event) events.Event {
					e.InvolvedObject.Namespace = "all-buckets"
					return e
				},
				forwarded: false,
			},
			{
				name: "drops event that is matched by exclusion",
				modifyEventFunc: func(e events.Event) events.Event {
					e.Message = "this is excluded"
					return e
				},
				forwarded: false,
			},
		}

		for _, tt := range tests {
			event = tt.modifyEventFunc(event)
			testSent()
			if tt.forwarded {
				testForwarded()
			} else {
				testFiltered()
			}
			req = nil
		}
	})
}
