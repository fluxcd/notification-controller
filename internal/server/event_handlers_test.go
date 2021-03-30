/*
Copyright 2020 The Flux authors

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
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	notificationv1beta1 "github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/internal/notifier"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/events"
)

const (
	testNamespace = "testing"
	sharedSecret  = "topsecret"
)

func TestEventServer_handleEvent_with_signing_secret(t *testing.T) {
	secret := makeTestSecret(sharedSecret)
	evt := makeTestEvent()
	evtBody := testMarshalEvent(t, evt)
	received, webhookServer := makeWebhookServer(t, true)
	provider := makeTestProvider(secret, webhookServer.URL)
	ts := makeTestEventServer(t, secret, provider, makeAlertForEvent(evt, provider))

	req := makeClientRequest(t, ts, "/", evtBody)

	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}

	assertResponseReceived(t, received, res)
}

func TestEventServer_handleEvent_invalid_signing_secret(t *testing.T) {
	secret := makeTestSecret("doesnotmatch")
	evt := makeTestEvent()
	evtBody := testMarshalEvent(t, evt)
	received, webhookServer := makeWebhookServer(t, false)
	provider := makeTestProvider(secret, webhookServer.URL)
	ts := makeTestEventServer(t, secret, provider, makeAlertForEvent(evt, provider))

	req := makeClientRequest(t, ts, "/", evtBody)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}

	assertResponseReceived(t, received, res)
}

func TestEventServer_handleEvent_no_signing_secret(t *testing.T) {
	evt := makeTestEvent()
	evtBody := testMarshalEvent(t, evt)
	received, webhookServer := makeWebhookServer(t, false)
	provider := makeTestProvider(nil, webhookServer.URL)
	ts := makeTestEventServer(t, provider, makeAlertForEvent(evt, provider))

	req := makeClientRequest(t, ts, "/", evtBody)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}

	assertResponseReceived(t, received, res)
}

func makeTestProvider(s *corev1.Secret, url string) *notificationv1beta1.Provider {
	var secretRef *meta.LocalObjectReference
	if s != nil {
		secretRef = &meta.LocalObjectReference{
			Name: s.ObjectMeta.Name,
		}
	}
	return &notificationv1beta1.Provider{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-provider",
			Namespace: testNamespace,
		},
		Spec: notificationv1beta1.ProviderSpec{
			Type:      "generic",
			Address:   url,
			SecretRef: secretRef,
		},
	}
}

func makeTestSecret(s string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testing-secret",
			Namespace: testNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"signing": []byte(s),
		},
	}
}

func makeTestEvent() events.Event {
	return events.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:            "GitRepository",
			Name:            "flux-system",
			Namespace:       "flux-system",
			UID:             "cc4d0095-83f4-4f08-98f2-d2e9f3731fb9",
			APIVersion:      "source.toolkit.fluxcd.io/v1beta1",
			ResourceVersion: "56921",
		},
		Severity:            "info",
		Timestamp:           metav1.Now(),
		Message:             "Fetched revision: main/731f7eaddfb6af01cb2173e18f0f75b0ba780ef1",
		Reason:              "info",
		ReportingController: "source-controller",
		ReportingInstance:   "source-controller-7c7b47f5f-8bhrp",
	}
}

func makeAlertForEvent(e events.Event, provider *notificationv1beta1.Provider) *notificationv1beta1.Alert {
	return &notificationv1beta1.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testing",
			Namespace: testNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "notification.toolkit.fluxcd.io/v1beta1",
			Kind:       "Alert",
		},
		Spec: notificationv1beta1.AlertSpec{
			ProviderRef: meta.LocalObjectReference{
				Name: provider.ObjectMeta.Name,
			},
			EventSeverity: e.Severity,
			EventSources: []notificationv1beta1.CrossNamespaceObjectReference{
				{
					Kind:      e.InvolvedObject.Kind,
					Name:      e.InvolvedObject.Name,
					Namespace: e.InvolvedObject.Namespace,
				},
			},
		},
		Status: notificationv1beta1.AlertStatus{
			Conditions: []metav1.Condition{
				{Type: meta.ReadyCondition, Status: metav1.ConditionTrue},
			},
		},
	}
}

func testMarshalEvent(t *testing.T, e events.Event) []byte {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func makeClientRequest(t *testing.T, ts *httptest.Server, path string, body []byte) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s%s", ts.URL, path), bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func makeTestEventServer(t *testing.T, objs ...runtime.Object) *httptest.Server {
	t.Helper()
	s := scheme.Scheme
	if err := notificationv1beta1.AddToScheme(s); err != nil {
		t.Fatalf("failed to register scheme: %s", err)
	}
	srv := NewEventServer("", logr.Discard(), fake.NewFakeClientWithScheme(s, objs...))
	ts := httptest.NewTLSServer(srv.makeMux())
	t.Cleanup(ts.Close)
	return ts
}

func assertResponseReceived(t *testing.T, c <-chan struct{}, res *http.Response) {
	t.Helper()
	w := time.After(5 * time.Second)
	select {
	case <-c:
		break
	case <-w:
		t.Fatalf("failed to receive a response")
	}
	if res.StatusCode != http.StatusAccepted {
		defer res.Body.Close()
		errMsg, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("didn't get a successful response: %v (%s)", res.StatusCode, strings.TrimSpace(string(errMsg)))
	}
}

func makeWebhookServer(t *testing.T, valid bool) (<-chan struct{}, *httptest.Server) {
	received := make(chan struct{})
	// This has to be an HTTP Server because the notifier can't use the
	// client from a TLSServer
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			close(received)
		}()
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read the request body: %s", err)
		}
		h := r.Header.Get(notifier.SignatureHeader)
		want := "sha256=" + sha256ForBody(t, b)
		if valid && h != want {
			t.Errorf("hmac verification failed, got %q, want %q", h, want)
		}
		if !valid && h == want {
			t.Error("hmac verification did not fail")
		}
	}))
	t.Cleanup(ts.Close)
	return received, ts
}

func sha256ForBody(t *testing.T, b []byte) string {
	t.Helper()
	hm := hmac.New(sha256.New, []byte(sharedSecret))
	_, err := hm.Write(b)
	if err != nil {
		t.Fatalf("failed to generate hmac for body: %s", err)
	}
	return hex.EncodeToString(hm.Sum(nil))

}
