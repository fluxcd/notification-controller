/*
Copyright 2023 The Flux authors

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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	"github.com/fluxcd/notification-controller/internal/notifier"
)

var fixedNow = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

func TestFilterAlertsForEvent(t *testing.T) {
	testNamespace := "foo-ns"

	testProvider := &apiv1beta3.Provider{}
	testProvider.Name = "provider-foo"
	testProvider.Namespace = testNamespace
	testProvider.Spec = apiv1beta3.ProviderSpec{
		Type:    "generic",
		Address: "https://example.com",
	}

	// Event involved object.
	involvedObj := corev1.ObjectReference{
		APIVersion: "kustomize.toolkit.fluxcd.io/v1",
		Kind:       "Kustomization",
		Name:       "foo",
		Namespace:  testNamespace,
	}
	testEvent := &eventv1.Event{
		InvolvedObject: involvedObj,
		Message:        "some excluded message",
	}

	tests := []struct {
		name             string
		alertSpecs       []apiv1beta3.AlertSpec
		resultAlertCount int
	}{
		{
			name: "all match",
			alertSpecs: []apiv1beta3.AlertSpec{
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "*",
						},
					},
				},
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "foo",
						},
					},
				},
			},
			resultAlertCount: 2,
		},
		{
			name: "some suspended alerts",
			alertSpecs: []apiv1beta3.AlertSpec{
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "*",
						},
					},
					Suspend: true,
				},
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "foo",
						},
					},
				},
			},
			resultAlertCount: 1,
		},
		{
			name: "alerts with inclusion list unmatch",
			alertSpecs: []apiv1beta3.AlertSpec{
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "*",
						},
					},
					InclusionList: []string{"some unmatch include"},
				},
			},
			resultAlertCount: 0,
		},
		{
			name: "alerts with inclusion list match",
			alertSpecs: []apiv1beta3.AlertSpec{
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "*",
						},
					},
					InclusionList: []string{"some unmatch include"},
				},
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "*",
						},
					},
					InclusionList: []string{"some"},
				},
			},
			resultAlertCount: 1,
		},
		{
			name: "alerts with invalid inclusion rule",
			alertSpecs: []apiv1beta3.AlertSpec{
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "*",
						},
					},
					InclusionList: []string{"["},
				},
			},
			resultAlertCount: 0,
		},
		{
			name: "alerts with exclusion list match",
			alertSpecs: []apiv1beta3.AlertSpec{
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "*",
						},
					},
				},
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "foo",
						},
					},
					ExclusionList: []string{"excluded message"},
				},
			},
			resultAlertCount: 1,
		},
		{
			name: "alerts with invalid exclusion rule",
			alertSpecs: []apiv1beta3.AlertSpec{
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "*",
						},
					},
				},
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "foo",
						},
					},
					ExclusionList: []string{"["},
				},
			},
			resultAlertCount: 2,
		},
		{
			name: "alerts with inclusion and exclusion list match",
			alertSpecs: []apiv1beta3.AlertSpec{
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "*",
						},
					},
				},
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: "Kustomization",
							Name: "foo",
						},
					},
					InclusionList: []string{"excluded message"},
					ExclusionList: []string{"excluded message"},
				},
			},
			resultAlertCount: 1,
		},
		{
			name: "event source NS is not overwritten by alert NS",
			alertSpecs: []apiv1beta3.AlertSpec{
				{
					EventSources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind:      "Kustomization",
							Name:      "*",
							Namespace: "foo-bar",
						},
					},
				},
			},
			resultAlertCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			alerts := []apiv1beta3.Alert{}
			for i, alertSpec := range tt.alertSpecs {
				// Add the default provider ref for this test.
				alertSpec.ProviderRef = meta.LocalObjectReference{Name: testProvider.Name}
				// Create new Alert with the spec.
				alert := apiv1beta3.Alert{}
				alert.Name = "test-alert-" + strconv.Itoa(i)
				alert.Namespace = testNamespace
				alert.Spec = alertSpec
				alerts = append(alerts, alert)
			}

			// Create fake objects and event server.
			scheme := runtime.NewScheme()
			g.Expect(apiv1beta3.AddToScheme(scheme)).ToNot(HaveOccurred())
			builder := fakeclient.NewClientBuilder().WithScheme(scheme)
			builder.WithObjects(testProvider)
			eventServer := EventServer{
				kubeClient:    builder.Build(),
				logger:        log.Log,
				EventRecorder: record.NewFakeRecorder(32),
			}

			result := eventServer.filterAlertsForEvent(context.TODO(), alerts, testEvent)
			g.Expect(len(result)).To(Equal(tt.resultAlertCount))
		})
	}
}

func TestDispatchNotification(t *testing.T) {
	testNamespace := "foo-ns"

	// Run test notification receiver server.
	rcvServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer rcvServer.Close()

	testProvider := &apiv1beta3.Provider{}
	testProvider.Name = "provider-foo"
	testProvider.Namespace = testNamespace
	testProvider.Spec = apiv1beta3.ProviderSpec{
		Type:    "generic",
		Address: rcvServer.URL,
	}

	testAlert := &apiv1beta3.Alert{}
	testAlert.Name = "alert-foo"
	testAlert.Namespace = testNamespace
	testAlert.Spec = apiv1beta3.AlertSpec{
		ProviderRef: meta.LocalObjectReference{Name: testProvider.Name},
	}

	// Event involved object.
	involvedObj := corev1.ObjectReference{
		APIVersion: "kustomize.toolkit.fluxcd.io/v1",
		Kind:       "Kustomization",
		Name:       "foo",
		Namespace:  testNamespace,
	}
	testEvent := &eventv1.Event{InvolvedObject: involvedObj}

	tests := []struct {
		name              string
		providerNamespace string
		providerSuspended bool
		wantErr           bool
	}{
		{
			name: "dispatch notification successfully",
		},
		{
			name:              "provider in different namespace",
			providerNamespace: "bar-ns",
			wantErr:           true,
		},
		{
			name:              "provider suspended, skip",
			providerSuspended: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			alert := testAlert.DeepCopy()
			provider := testProvider.DeepCopy()

			// Override the alert and provider with test parameters.
			if tt.providerNamespace != "" {
				provider.Namespace = tt.providerNamespace
			}
			provider.Spec.Suspend = tt.providerSuspended

			// Create fake objects and event server.
			scheme := runtime.NewScheme()
			g.Expect(apiv1beta3.AddToScheme(scheme)).ToNot(HaveOccurred())
			g.Expect(corev1.AddToScheme(scheme)).ToNot(HaveOccurred())
			builder := fakeclient.NewClientBuilder().WithScheme(scheme)
			builder.WithObjects(provider)
			eventServer := EventServer{
				kubeClient:    builder.Build(),
				logger:        log.Log,
				EventRecorder: record.NewFakeRecorder(32),
			}

			_, err := eventServer.dispatchNotification(context.TODO(), testEvent, alert)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func TestGetNotificationParams(t *testing.T) {
	testNamespace := "foo-ns"

	// Provider secret.
	testSecret := &corev1.Secret{}
	testSecret.Name = "secret-foo"
	testSecret.Namespace = testNamespace

	testProvider := &apiv1beta3.Provider{}
	testProvider.Name = "provider-foo"
	testProvider.Namespace = testNamespace
	testProvider.Spec = apiv1beta3.ProviderSpec{
		Type:      "generic",
		Address:   "https://example.com",
		SecretRef: &meta.LocalObjectReference{Name: testSecret.Name},
	}

	testAlert := &apiv1beta3.Alert{}
	testAlert.Name = "alert-foo"
	testAlert.Namespace = testNamespace
	testAlert.Spec = apiv1beta3.AlertSpec{
		ProviderRef: meta.LocalObjectReference{Name: testProvider.Name},
	}

	// Event involved object.
	involvedObj := corev1.ObjectReference{
		APIVersion: "kustomize.toolkit.fluxcd.io/v1",
		Kind:       "Kustomization",
		Name:       "foo",
		Namespace:  testNamespace,
	}
	testEvent := &eventv1.Event{InvolvedObject: involvedObj}

	tests := []struct {
		name                    string
		alertNamespace          string
		alertSummary            string
		alertEventMetadata      map[string]string
		providerType            string
		providerNamespace       string
		providerSuspended       bool
		providerServiceAccount  string
		secretNamespace         string
		noCrossNSRefs           bool
		enableObjLevelWI        bool
		eventMetadata           map[string]string
		wantErr                 bool
		wantDroppedCommitStatus bool
	}{
		{
			name:              "event src and alert in diff NS",
			alertNamespace:    "bar-ns",
			providerNamespace: "bar-ns",
			secretNamespace:   "bar-ns",
		},
		{
			name:              "event src and alert in diff NS with no cross NS refs",
			alertNamespace:    "bar-ns",
			providerNamespace: "bar-ns",
			noCrossNSRefs:     true,
			wantErr:           true,
		},
		{
			name:              "provider not found",
			providerNamespace: "bar-ns",
			wantErr:           true,
		},
		{
			name:              "provider secret in diff NS but provider suspended",
			providerSuspended: true,
			secretNamespace:   "bar-ns",
		},
		{
			name:            "provider secret in different NS, fail to create notifier",
			secretNamespace: "bar-ns",
			wantErr:         true,
		},
		{
			name:         "alert with summary, no event metadata",
			alertSummary: "some summary text",
		},
		{
			name:         "alert with summary, with event metadata",
			alertSummary: "some summary text",
			eventMetadata: map[string]string{
				"foo":     "bar",
				"summary": "baz",
			},
		},
		{
			name: "alert with event metadata",
			alertEventMetadata: map[string]string{
				"aaa": "bbb",
				"ccc": "ddd",
			},
		},
		{
			name:                   "object level workload identity feature gate disabled",
			providerServiceAccount: "foo",
			enableObjLevelWI:       false,
			wantErr:                true,
		},
		{
			name:                   "object level workload identity feature gate enabled",
			providerServiceAccount: "foo",
			enableObjLevelWI:       true,
			wantErr:                false,
		},
		{
			name:                    "commit status provider drops event without commit key",
			providerType:            apiv1beta3.GitHubProvider,
			wantDroppedCommitStatus: true,
		},
		{
			name:         "commit status provider does not drop commit status update event without commit key",
			providerType: apiv1beta3.GitHubProvider,
			eventMetadata: map[string]string{
				"kustomize.toolkit.fluxcd.io/" + eventv1.MetaCommitStatusKey: eventv1.MetaCommitStatusUpdateValue,
			},
			wantErr: true, // proceeds past the guard and fails on notifier creation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			alert := testAlert.DeepCopy()
			provider := testProvider.DeepCopy()
			secret := testSecret.DeepCopy()
			event := testEvent.DeepCopy()

			// Override the alert, provider, secret and event with test
			// parameters.
			if tt.alertNamespace != "" {
				alert.Namespace = tt.alertNamespace
			}
			if tt.alertSummary != "" {
				alert.Spec.Summary = tt.alertSummary
			}
			if tt.alertEventMetadata != nil {
				alert.Spec.EventMetadata = tt.alertEventMetadata
			}
			if tt.providerType != "" {
				provider.Spec.Type = tt.providerType
			}
			if tt.providerNamespace != "" {
				provider.Namespace = tt.providerNamespace
			}
			provider.Spec.Suspend = tt.providerSuspended
			provider.Spec.ServiceAccountName = tt.providerServiceAccount
			if tt.secretNamespace != "" {
				secret.Namespace = tt.secretNamespace
			}
			if tt.eventMetadata != nil {
				event.Metadata = tt.eventMetadata
			}

			if tt.enableObjLevelWI {
				auth.EnableObjectLevelWorkloadIdentity()
				t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)
			}

			// Create fake objects and event server.
			scheme := runtime.NewScheme()
			g.Expect(apiv1beta3.AddToScheme(scheme)).ToNot(HaveOccurred())
			g.Expect(corev1.AddToScheme(scheme)).ToNot(HaveOccurred())
			builder := fakeclient.NewClientBuilder().WithScheme(scheme)
			builder.WithObjects(provider, secret)
			eventServer := EventServer{
				kubeClient:           builder.Build(),
				logger:               log.Log,
				noCrossNamespaceRefs: tt.noCrossNSRefs,
				EventRecorder:        record.NewFakeRecorder(32),
			}

			params, dropped, err := eventServer.getNotificationParams(context.TODO(), event, alert)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			g.Expect(dropped.commitStatus).To(Equal(tt.wantDroppedCommitStatus))
			if tt.alertSummary != "" {
				g.Expect(params.event.Metadata["summary"]).To(Equal(tt.alertSummary))
			}
			// NOTE: This is performing simple check. Thorough test for event
			// metadata is performed in TestCombineEventMetadata.
			if tt.alertEventMetadata != nil {
				for k, v := range tt.alertEventMetadata {
					g.Expect(params.event.Metadata).To(HaveKeyWithValue(k, v))
				}
			}
		})
	}
}

func TestCreateNotifier(t *testing.T) {
	secretName := "foo-secret"
	certSecretName := "cert-secret"
	proxySecretName := "proxy-secret"

	// Generate test certificates for mTLS testing
	caCert, clientCert, clientKey := generateTestCertificates(t)

	// Helper to create expected TLS configs
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caCert)

	clientCertPair, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatalf("Failed to create client cert pair: %v", err)
	}

	tests := []struct {
		name            string
		providerSpec    *apiv1beta3.ProviderSpec
		secretType      corev1.SecretType
		secretData      map[string][]byte
		certSecretData  map[string][]byte
		proxySecretData map[string][]byte
		wantErr         bool
		wantTLSConfig   *tls.Config
	}{
		{
			name: "valid address, no secret ref",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:    "slack",
				Address: "https://example.com",
			},
		},
		{
			name: "reference to non-existing secret ref",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:      "slack",
				SecretRef: &meta.LocalObjectReference{Name: "foo"},
			},
			wantErr: true,
		},
		// TODO: Remove deprecated secret proxy key tests when Provider v1 is released.
		{
			name: "reference to secret with valid address, proxy, headers",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:      "slack",
				SecretRef: &meta.LocalObjectReference{Name: secretName},
			},
			secretData: map[string][]byte{
				"address": []byte("https://example.com"),
				"proxy":   []byte("https://exampleproxy.com"),
				"headers": []byte(`foo: bar`),
			},
		},
		{
			name: "reference to secret with invalid proxy",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:      "slack",
				SecretRef: &meta.LocalObjectReference{Name: secretName},
			},
			secretData: map[string][]byte{
				"address": []byte("https://example.com"),
				"proxy":   []byte("https://exampleproxy.com|"),
			},
			wantErr: true,
		},
		{
			name: "invalid headers in secret reference",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:      "slack",
				SecretRef: &meta.LocalObjectReference{Name: secretName},
			},
			secretData: map[string][]byte{
				"address": []byte("https://example.com"),
				"headers": []byte("foo"),
			},
			wantErr: true,
		},
		{
			name: "invalid spec address overridden by valid secret ref address",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:      "slack",
				SecretRef: &meta.LocalObjectReference{Name: secretName},
				Address:   "https://example.com|",
			},
			secretData: map[string][]byte{
				"address": []byte("https://example.com"),
			},
		},
		// TODO: Remove deprecated spec.proxy field tests when Provider v1 is released.
		{
			name: "invalid spec proxy overridden by valid secret ref proxy",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:      "slack",
				SecretRef: &meta.LocalObjectReference{Name: secretName},
				Proxy:     "https://example.com|",
			},
			secretData: map[string][]byte{
				"address": []byte("https://example.com"),
				"proxy":   []byte("https://example.com"),
			},
		},
		{
			name: "reference to unsupported cert secret type",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "slack",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			secretType: corev1.SecretTypeDockercfg,
			certSecretData: map[string][]byte{
				"aaa": []byte("bbb"),
			},
			wantErr: true,
		},
		{
			name: "reference to non-existing cert secret",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "slack",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: "some-secret"},
			},
			wantErr: true,
		},
		{
			name: "reference to cert secret without cert data",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "slack",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			certSecretData: map[string][]byte{
				"aaa": []byte("bbb"),
			},
			wantErr: true,
		},
		{
			name: "cert secret reference in ca.crt with valid CA",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "slack",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			certSecretData: map[string][]byte{
				"ca.crt": caCert,
			},
			wantTLSConfig: &tls.Config{RootCAs: caPool},
		},
		{
			name: "cert secret reference in caFile with valid CA",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "slack",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			certSecretData: map[string][]byte{
				"caFile": caCert,
			},
			wantTLSConfig: &tls.Config{RootCAs: caPool},
		},
		{
			name: "cert secret reference in both ca.crt and caFile",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "slack",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			certSecretData: map[string][]byte{
				// Based on https://pkg.go.dev/crypto/tls#X509KeyPair example.
				"ca.crt": []byte(`-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`),
				"caFile": []byte(`aaaaa`), // invalid
			},
		},
		{
			name: "cert secret reference with invalid CA",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "slack",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			certSecretData: map[string][]byte{
				"ca.crt": []byte(`aaaaa`),
			},
			wantErr: true,
		},
		{
			name: "mTLS with standard keys",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "generic",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			certSecretData: map[string][]byte{
				"ca.crt":  caCert,
				"tls.crt": clientCert,
				"tls.key": clientKey,
			},
			wantTLSConfig: &tls.Config{RootCAs: caPool, Certificates: []tls.Certificate{clientCertPair}},
		},
		{
			name: "mTLS with legacy keys",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "generic",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			certSecretData: map[string][]byte{
				"caFile":   caCert,
				"certFile": clientCert,
				"keyFile":  clientKey,
			},
			wantTLSConfig: &tls.Config{RootCAs: caPool, Certificates: []tls.Certificate{clientCertPair}},
		},
		{
			name: "client cert without CA",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "generic",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			certSecretData: map[string][]byte{
				"tls.crt": clientCert,
				"tls.key": clientKey,
			},
			wantTLSConfig: &tls.Config{Certificates: []tls.Certificate{clientCertPair}},
		},
		{
			name: "unsupported provider",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:    "foo",
				Address: "https://example.com",
			},
			wantErr: true,
		},
		{
			name: "address in secret too long",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:      "msteams",
				SecretRef: &meta.LocalObjectReference{Name: secretName},
			},
			secretData: map[string][]byte{
				"address": []byte(fmt.Sprintf("https://example.org/%s", strings.Repeat("a", 2029))),
			},
			wantErr: true,
		},
		{
			name: "address in secret exactly as long as possible",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:      "msteams",
				SecretRef: &meta.LocalObjectReference{Name: secretName},
			},
			secretData: map[string][]byte{
				"address": []byte(fmt.Sprintf("https://example.org/%s", strings.Repeat("a", 2028))),
			},
			wantErr: false,
		},
		{
			name: "proxy from ProxySecretRef",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:           "generic",
				Address:        "https://example.com",
				ProxySecretRef: &meta.LocalObjectReference{Name: proxySecretName},
			},
			proxySecretData: map[string][]byte{
				"address": []byte("http://proxy.example.com:8080"),
			},
		},
		{
			name: "proxy from ProxySecretRef with authentication",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:           "generic",
				Address:        "https://example.com",
				ProxySecretRef: &meta.LocalObjectReference{Name: proxySecretName},
			},
			proxySecretData: map[string][]byte{
				"address":  []byte("http://proxy.example.com:8080"),
				"username": []byte("proxyuser"),
				"password": []byte("proxypass"),
			},
		},
		{
			name: "ProxySecretRef reference to non-existing secret",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:           "generic",
				Address:        "https://example.com",
				ProxySecretRef: &meta.LocalObjectReference{Name: "non-existing"},
			},
			wantErr: true,
		},
		{
			name: "ProxySecretRef missing address field",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:           "generic",
				Address:        "https://example.com",
				ProxySecretRef: &meta.LocalObjectReference{Name: proxySecretName},
			},
			proxySecretData: map[string][]byte{
				"username": []byte("proxyuser"),
			},
			wantErr: true,
		},
		// TODO: Remove deprecated spec.proxy field tests when Provider v1 is released.
		{
			name: "deprecated spec.proxy field",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:    "generic",
				Address: "https://example.com",
				Proxy:   "http://proxy.example.com:8080",
			},
		},
		{
			name: "provider type that does not require address field",
			providerSpec: &apiv1beta3.ProviderSpec{
				// Telegram generates URLs internally, so address field is not required
				Type:      "telegram",
				Channel:   "test-channel",
				SecretRef: &meta.LocalObjectReference{Name: secretName},
			},
			secretData: map[string][]byte{
				"token": []byte("test-token"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create fake objects and event server.
			scheme := runtime.NewScheme()
			g.Expect(apiv1beta3.AddToScheme(scheme)).ToNot(HaveOccurred())
			g.Expect(corev1.AddToScheme(scheme)).ToNot(HaveOccurred())
			builder := fakeclient.NewClientBuilder().WithScheme(scheme)
			if tt.secretData != nil {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: secretName},
					Data:       tt.secretData,
				}
				builder.WithObjects(secret)
			}
			if tt.certSecretData != nil {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: certSecretName},
					Type:       tt.secretType,
					Data:       tt.certSecretData,
				}
				builder.WithObjects(secret)
			}
			if tt.proxySecretData != nil {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: proxySecretName},
					Data:       tt.proxySecretData,
				}
				builder.WithObjects(secret)
			}
			provider := apiv1beta3.Provider{Spec: *tt.providerSpec}

			notifier, _, err := createNotifier(context.TODO(), builder.Build(), &provider, "", nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))

			if !tt.wantErr && tt.wantTLSConfig != nil {
				g.Expect(notifier).ToNot(BeNil(), "Expected notifier to be created but got nil")

				// Get TLS configuration from notifier
				tlsConfig := getNotifierTLSConfig(notifier)
				if tlsConfig == nil {
					// Notifier doesn't support TLS via postMessage pattern, skip the check
					return
				}

				g.Expect(tlsConfig).ToNot(BeNil(), "Expected TLS configuration but got nil")
				if tt.wantTLSConfig.RootCAs != nil {
					g.Expect(tlsConfig.RootCAs).ToNot(BeNil())
				} else {
					g.Expect(tlsConfig.RootCAs).To(BeNil())
				}

				g.Expect(tlsConfig.Certificates).To(HaveLen(len(tt.wantTLSConfig.Certificates)))
				if len(tt.wantTLSConfig.Certificates) > 0 {
					g.Expect(tlsConfig.Certificates[0]).To(Equal(tt.wantTLSConfig.Certificates[0]))
				}
			}
		})
	}
}

func TestCreateCommitStatus(t *testing.T) {
	tests := []struct {
		name             string
		provider         apiv1beta3.Provider
		notification     eventv1.Event
		alert            *apiv1beta3.Alert
		wantCommitStatus string
		wantErr          bool
	}{
		{
			name: "non-git provider: slack",
			provider: apiv1beta3.Provider{
				Spec: apiv1beta3.ProviderSpec{
					Type: "slack",
				},
			},
			wantCommitStatus: "",
		},
		{
			name: "non-git provider: msteams",
			provider: apiv1beta3.Provider{
				Spec: apiv1beta3.ProviderSpec{
					Type: "msteams",
				},
			},
			wantCommitStatus: "",
		},
		{
			name: "git provider without commit status expression",
			provider: apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					UID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				},
				Spec: apiv1beta3.ProviderSpec{
					Type: "github",
				},
			},
			notification: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Reason: "ApplySucceeded",
			},
			wantCommitStatus: "kustomization/gitops-system/0c9c2e41",
		},
		{
			name: "gitlab provider without commit status expression",
			provider: apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					UID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				},
				Spec: apiv1beta3.ProviderSpec{
					Type: "gitlab",
				},
			},
			notification: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "HelmRelease",
					Name: "gitops-system",
				},
				Reason: "ApplySucceeded",
			},
			wantCommitStatus: "helmrelease/gitops-system/0c9c2e41",
		},
		{
			name: "git provider with commit status expression",
			provider: apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					UID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				},
				Spec: apiv1beta3.ProviderSpec{
					Type:             "github",
					CommitStatusExpr: "event.involvedObject.kind + '/' + event.involvedObject.name + '/' + event.metadata.environment + '/' + provider.metadata.uid",
				},
			},
			notification: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Reason: "ApplySucceeded",
				Metadata: map[string]string{
					"environment": "production",
				},
			},
			alert: &apiv1beta3.Alert{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-alert",
				},
			},
			wantCommitStatus: "Kustomization/gitops-system/production/0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
		},
		{
			name: "git provider with commit status expression using first value of provider UID",
			provider: apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					UID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				},
				Spec: apiv1beta3.ProviderSpec{
					Type:             "github",
					CommitStatusExpr: "event.involvedObject.kind + '/' + event.involvedObject.name + '/' + event.metadata.environment + '/' + provider.metadata.uid.split('-').first().value()",
				},
			},
			notification: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					"environment": "production",
				},
			},
			alert: &apiv1beta3.Alert{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-alert",
				},
			},
			wantCommitStatus: "Kustomization/gitops-system/production/0c9c2e41",
		},
		{
			name: "git provider with commit status expression using event, alert, and provider",
			provider: apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					UID: "0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a",
				},
				Spec: apiv1beta3.ProviderSpec{
					Type:             "github",
					CommitStatusExpr: "event.involvedObject.kind + '/' + event.involvedObject.name + '/' + event.metadata.environment + '/' + provider.metadata.uid + '/' + alert.metadata.name",
				},
			},
			notification: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					"environment": "production",
				},
			},
			alert: &apiv1beta3.Alert{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-alert",
				},
			},
			wantCommitStatus: "Kustomization/gitops-system/production/0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a/test-alert",
		},
		{
			name: "git provider with invalid commit status expression referencing non-existent event metadata",
			notification: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					"foo": "bar",
				},
			},
			provider: apiv1beta3.Provider{
				Spec: apiv1beta3.ProviderSpec{
					Type:             "github",
					CommitStatusExpr: "event.involvedObject.kind + '/' + event.involvedObject.name + '/' + event.metadata.notfound + '/' + provider.metadata.uid",
				},
			},
			wantErr: true,
		},
		{
			name: "git provider with invalid commit status expression",
			notification: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					"foo": "bar",
				},
			},
			provider: apiv1beta3.Provider{
				Spec: apiv1beta3.ProviderSpec{
					Type:             "github",
					CommitStatusExpr: "event.involvedObject.kind == 'Kustomization'",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create fake objects and event server.
			scheme := runtime.NewScheme()
			g.Expect(apiv1beta3.AddToScheme(scheme)).ToNot(HaveOccurred())
			g.Expect(corev1.AddToScheme(scheme)).ToNot(HaveOccurred())

			commitStatus, err := createCommitStatus(context.TODO(), &tt.provider, &tt.notification, tt.alert)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			g.Expect(commitStatus).To(Equal(tt.wantCommitStatus))
		})
	}
}

func TestEventMatchesAlert(t *testing.T) {
	testNamespace := "foo-ns"
	involvedObj := corev1.ObjectReference{
		APIVersion: "kustomize.toolkit.fluxcd.io/v1",
		Kind:       "Kustomization",
		Name:       "foo",
		Namespace:  testNamespace,
	}

	tests := []struct {
		name          string
		event         *eventv1.Event
		source        apiv1.CrossNamespaceObjectReference
		severity      string
		resourcesFile string
		wantResult    bool
	}{
		{
			name:  "source and event namespace mismatch",
			event: &eventv1.Event{InvolvedObject: involvedObj},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "Kustomization",
				Name:      "*",
				Namespace: "test-ns",
			},
			severity:   "info",
			wantResult: false,
		},
		{
			name:  "source and event kind mismatch",
			event: &eventv1.Event{InvolvedObject: involvedObj},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "GitRepository",
				Name:      "*",
				Namespace: testNamespace,
			},
			severity:   "info",
			wantResult: false,
		},
		{
			name: "event and alert severity mismatch, alert severity error",
			event: &eventv1.Event{
				InvolvedObject: involvedObj,
				Severity:       "info",
			},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "Kustomization",
				Name:      "*",
				Namespace: testNamespace,
			},
			severity:   "error",
			wantResult: false,
		},
		{
			name: "event and alert severity mismatch, alert severity info",
			event: &eventv1.Event{
				InvolvedObject: involvedObj,
				Severity:       "error",
			},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "Kustomization",
				Name:      "*",
				Namespace: testNamespace,
			},
			severity:   "info",
			wantResult: true,
		},
		{
			name:  "source with matching kind and namespace, any name",
			event: &eventv1.Event{InvolvedObject: involvedObj},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "Kustomization",
				Name:      "*",
				Namespace: testNamespace,
			},
			severity:   "info",
			wantResult: true,
		},
		{
			name:  "source with matching kind and namespace, unmatched name",
			event: &eventv1.Event{InvolvedObject: involvedObj},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "Kustomization",
				Name:      "bar",
				Namespace: testNamespace,
			},
			severity:   "info",
			wantResult: false,
		},
		{
			name:  "source with matching kind and namespace, matched name",
			event: &eventv1.Event{InvolvedObject: involvedObj},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "Kustomization",
				Name:      "foo",
				Namespace: testNamespace,
			},
			severity:   "info",
			wantResult: true,
		},
		{
			name:          "label selector match",
			resourcesFile: "./testdata/kustomization.yaml",
			event:         &eventv1.Event{InvolvedObject: involvedObj},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "Kustomization",
				Name:      "*",
				Namespace: testNamespace,
				MatchLabels: map[string]string{
					"app": "podinfo",
				},
			},
			severity:   "info",
			wantResult: true,
		},
		{
			name:          "label selector mismatch",
			resourcesFile: "./testdata/kustomization.yaml",
			event:         &eventv1.Event{InvolvedObject: involvedObj},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "Kustomization",
				Name:      "*",
				Namespace: testNamespace,
				MatchLabels: map[string]string{
					"aaa": "bbb",
				},
			},
			severity:   "info",
			wantResult: false,
		},
		{
			name:  "label selector, object not found",
			event: &eventv1.Event{InvolvedObject: involvedObj},
			source: apiv1.CrossNamespaceObjectReference{
				Kind:      "Kustomization",
				Name:      "*",
				Namespace: testNamespace,
				MatchLabels: map[string]string{
					"aaa": "bbb",
				},
			},
			severity:   "info",
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(apiv1beta3.AddToScheme(scheme)).ToNot(HaveOccurred())

			builder := fakeclient.NewClientBuilder().WithScheme(scheme)

			// Create pre-existing resource from manifest file.
			if tt.resourcesFile != "" {
				obj, err := readManifest(tt.resourcesFile, testNamespace)
				g.Expect(err).ToNot(HaveOccurred())
				builder.WithObjects(obj)
			}

			eventServer := EventServer{
				kubeClient:    builder.Build(),
				logger:        log.Log,
				EventRecorder: record.NewFakeRecorder(32),
			}
			alert := &apiv1beta3.Alert{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-alert",
					Namespace: "test-ns",
				},
				Spec: apiv1beta3.AlertSpec{
					EventSeverity: tt.severity,
				},
			}

			result := eventServer.eventMatchesAlertSource(context.TODO(), tt.event, alert, tt.source)
			g.Expect(result).To(Equal(tt.wantResult))
		})
	}
}

func TestCombineEventMetadata(t *testing.T) {
	for name, tt := range map[string]struct {
		event            eventv1.Event
		alert            apiv1beta3.Alert
		expectedMetadata map[string]string
		conflictEvent    string
	}{
		"empty metadata": {
			event:            eventv1.Event{},
			alert:            apiv1beta3.Alert{},
			expectedMetadata: nil,
		},
		"all metadata sources work": {
			event: eventv1.Event{
				Metadata: map[string]string{
					"kustomize.toolkit.fluxcd.io/controllerMetadata1": "controllerMetadataValue1",
					"kustomize.toolkit.fluxcd.io/controllerMetadata2": "controllerMetadataValue2",
					"event.toolkit.fluxcd.io/objectMetadata1":         "objectMetadataValue1",
					"event.toolkit.fluxcd.io/objectMetadata2":         "objectMetadataValue2",
				},
			},
			alert: apiv1beta3.Alert{
				Spec: apiv1beta3.AlertSpec{
					Summary: "summaryValue",
					EventMetadata: map[string]string{
						"foo": "bar",
						"baz": "qux",
					},
				},
			},
			expectedMetadata: map[string]string{
				"foo":                 "bar",
				"baz":                 "qux",
				"controllerMetadata1": "controllerMetadataValue1",
				"controllerMetadata2": "controllerMetadataValue2",
				"summary":             "summaryValue",
				"objectMetadata1":     "objectMetadataValue1",
				"objectMetadata2":     "objectMetadataValue2",
			},
		},
		"object metadata is overriden by summary": {
			event: eventv1.Event{
				Metadata: map[string]string{
					"event.toolkit.fluxcd.io/summary": "objectSummary",
				},
			},
			alert: apiv1beta3.Alert{
				Spec: apiv1beta3.AlertSpec{
					Summary: "alertSummary",
				},
			},
			expectedMetadata: map[string]string{
				"summary": "alertSummary",
			},
			conflictEvent: "Warning MetadataAppendFailed metadata key conflicts detected (please refer to the Alert API docs and Flux RFC 0008 for more information) map[summary:involved object annotations, Alert object .spec.summary]",
		},
		"alert event metadata is overriden by summary": {
			event: eventv1.Event{},
			alert: apiv1beta3.Alert{
				Spec: apiv1beta3.AlertSpec{
					Summary: "alertSummary",
					EventMetadata: map[string]string{
						"summary": "eventMetadataSummary",
					},
				},
			},
			expectedMetadata: map[string]string{
				"summary": "alertSummary",
			},
			conflictEvent: "Warning MetadataAppendFailed metadata key conflicts detected (please refer to the Alert API docs and Flux RFC 0008 for more information) map[summary:Alert object .spec.eventMetadata, Alert object .spec.summary]",
		},
		"summary is overriden by controller metadata": {
			event: eventv1.Event{
				Metadata: map[string]string{
					"kustomize.toolkit.fluxcd.io/summary": "controllerSummary",
				},
			},
			alert: apiv1beta3.Alert{
				Spec: apiv1beta3.AlertSpec{
					Summary: "alertSummary",
				},
			},
			expectedMetadata: map[string]string{
				"summary": "controllerSummary",
			},
			conflictEvent: "Warning MetadataAppendFailed metadata key conflicts detected (please refer to the Alert API docs and Flux RFC 0008 for more information) map[summary:Alert object .spec.summary, involved object controller metadata]",
		},
		"precedence order in RFC 0008 is honered": {
			event: eventv1.Event{
				Metadata: map[string]string{
					"kustomize.toolkit.fluxcd.io/objectMetadataOverridenByController": "controllerMetadataValue1",
					"kustomize.toolkit.fluxcd.io/alertMetadataOverridenByController":  "controllerMetadataValue2",
					"kustomize.toolkit.fluxcd.io/controllerMetadata":                  "controllerMetadataValue3",
					"event.toolkit.fluxcd.io/objectMetadata":                          "objectMetadataValue1",
					"event.toolkit.fluxcd.io/objectMetadataOverridenByAlert":          "objectMetadataValue2",
					"event.toolkit.fluxcd.io/objectMetadataOverridenByController":     "objectMetadataValue3",
				},
			},
			alert: apiv1beta3.Alert{
				Spec: apiv1beta3.AlertSpec{
					EventMetadata: map[string]string{
						"objectMetadataOverridenByAlert":     "alertMetadataValue1",
						"alertMetadata":                      "alertMetadataValue2",
						"alertMetadataOverridenByController": "alertMetadataValue3",
					},
				},
			},
			expectedMetadata: map[string]string{
				"objectMetadata":                      "objectMetadataValue1",
				"objectMetadataOverridenByAlert":      "alertMetadataValue1",
				"objectMetadataOverridenByController": "controllerMetadataValue1",
				"alertMetadata":                       "alertMetadataValue2",
				"alertMetadataOverridenByController":  "controllerMetadataValue2",
				"controllerMetadata":                  "controllerMetadataValue3",
			},
			conflictEvent: "Warning MetadataAppendFailed metadata key conflicts detected (please refer to the Alert API docs and Flux RFC 0008 for more information) map[alertMetadataOverridenByController:Alert object .spec.eventMetadata, involved object controller metadata objectMetadataOverridenByAlert:involved object annotations, Alert object .spec.eventMetadata objectMetadataOverridenByController:involved object annotations, involved object controller metadata]",
		},
	} {
		t.Run(name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			eventRecorder := record.NewFakeRecorder(1)
			s := &EventServer{
				logger:        log.Log,
				EventRecorder: eventRecorder,
			}

			tt.event.InvolvedObject.APIVersion = "kustomize.toolkit.fluxcd.io/v1"
			s.combineEventMetadata(context.Background(), &tt.event, &tt.alert)
			g.Expect(tt.event.Metadata).To(BeEquivalentTo(tt.expectedMetadata))

			var event string
			select {
			case event = <-eventRecorder.Events:
			default:
			}
			g.Expect(event).To(Equal(tt.conflictEvent))
		})
	}
}

func Test_excludeInternalMetadata(t *testing.T) {
	tests := []struct {
		name         string
		event        eventv1.Event
		wantMetadata map[string]string
	}{
		{
			name: "no metadata",
		},
		{
			name: "internal metadata",
			event: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{
					APIVersion: "kustomize.toolkit.fluxcd.io/v1",
				},
				Metadata: map[string]string{
					"kustomize.toolkit.fluxcd.io/" + eventv1.MetaTokenKey:    "aaaa",
					"kustomize.toolkit.fluxcd.io/" + eventv1.MetaRevisionKey: "bbbb",
				},
			},
			wantMetadata: map[string]string{
				"kustomize.toolkit.fluxcd.io/" + eventv1.MetaRevisionKey: "bbbb",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			excludeInternalMetadata(&tt.event)
			g.Expect(tt.event.Metadata).To(BeEquivalentTo(tt.wantMetadata))
		})
	}
}

func TestGetTLSConfigForProvider(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Reuse your existing helper.
	caCert, clientCert, clientKey := generateTestCertificates(t)

	// Expected TLS pieces.
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caCert)

	clientCertPair, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatalf("failed to create client cert pair: %v", err)
	}

	getSecret := func(name string, data map[string][]byte) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Data:       data,
		}
	}

	tests := []struct {
		name               string
		providerType       string
		providerCertSecret *corev1.Secret
		providerSecret     *corev1.Secret
		wantErr            bool
		wantTLSConfig      *tls.Config
	}{
		{
			name:         "no secrets returns nil TLS config",
			providerType: apiv1beta3.GitHubProvider,
		},
		{
			name:               "providerCertSecret in ca.crt with valid CA",
			providerType:       apiv1beta3.GitHubProvider,
			providerCertSecret: getSecret("cert-secret", map[string][]byte{"ca.crt": caCert}),
			wantTLSConfig:      &tls.Config{RootCAs: caPool},
		},
		{
			name:               "providerCertSecret precedence over providerSecret",
			providerType:       apiv1beta3.GitHubProvider,
			providerCertSecret: getSecret("cert-secret", map[string][]byte{"ca.crt": caCert}),
			// Intentionally invalid providerSecret to prove precedence is honored.
			providerSecret: getSecret("ignored", map[string][]byte{"ca.crt": []byte("not-a-cert")}),
			wantTLSConfig:  &tls.Config{RootCAs: caPool},
		},
		{
			name:               "providerCertSecret with invalid CA returns error",
			providerType:       apiv1beta3.GitHubProvider,
			providerCertSecret: getSecret("cert-secret", map[string][]byte{"ca.crt": []byte("bogus")}),
			wantErr:            true,
		},
		{
			name:           "providerSecret in ca.crt with valid CA (git provider)",
			providerType:   apiv1beta3.GitHubProvider,
			providerSecret: getSecret("git-secret", map[string][]byte{"ca.crt": caCert}),
			wantTLSConfig:  &tls.Config{RootCAs: caPool},
		},
		{
			name:           "providerSecret without ca.crt (git provider) returns nil TLS config",
			providerType:   apiv1beta3.GitHubProvider,
			providerSecret: getSecret("git-secret-no-ca", map[string][]byte{"foo": []byte("bar")}),
		},
		{
			name:           "providerSecret in ca.crt with invalid CA (git provider) returns error",
			providerType:   apiv1beta3.GitHubProvider,
			providerSecret: getSecret("git-secret", map[string][]byte{"ca.crt": []byte("aaa")}),
			wantErr:        true,
		},
		{
			name:           "providerSecret ignored for non-git provider even if ca.crt present",
			providerType:   apiv1beta3.SlackProvider,
			providerSecret: getSecret("non-git", map[string][]byte{"ca.crt": caCert}),
		},
		{
			name:               "providerCertSecret with (mTLS)",
			providerType:       apiv1beta3.SlackProvider,
			providerCertSecret: getSecret("cert-secret", map[string][]byte{"ca.crt": caCert, "tls.crt": clientCert, "tls.key": clientKey}),
			wantTLSConfig:      &tls.Config{RootCAs: caPool, Certificates: []tls.Certificate{clientCertPair}},
		},
		{
			name:           "providerSecret with (mTLS)",
			providerType:   apiv1beta3.GitHubProvider,
			providerSecret: getSecret("cert-secret", map[string][]byte{"ca.crt": caCert, "tls.crt": clientCert, "tls.key": clientKey}),
			wantTLSConfig:  &tls.Config{RootCAs: caPool, Certificates: []tls.Certificate{clientCertPair}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := getTLSConfigForProvider(ctx, tt.providerCertSecret, tt.providerSecret, tt.providerType)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			if tt.wantTLSConfig == nil {
				g.Expect(got).To(BeNil(), "expected nil TLS config")
				return
			}

			g.Expect(got).ToNot(BeNil(), "expected non-nil TLS config")

			// RootCAs presence matches expectation.
			if tt.wantTLSConfig.RootCAs != nil {
				g.Expect(got.RootCAs).ToNot(BeNil())
			} else {
				g.Expect(got.RootCAs).To(BeNil())
			}

			// Certificates (mTLS) presence matches expectation.
			g.Expect(got.Certificates).To(HaveLen(len(tt.wantTLSConfig.Certificates)))
			if len(tt.wantTLSConfig.Certificates) > 0 {
				// Basic sanity: leaf cert is parsable; avoids brittle struct equality.
				g.Expect(got.Certificates[0].Certificate).ToNot(BeNil())
				_, parseErr := x509.ParseCertificate(got.Certificates[0].Certificate[0])
				g.Expect(parseErr).ToNot(HaveOccurred())
			}
		})
	}
}

// generateTestCertificates generates test certificates for mTLS testing.
// TODO: Move to pkg/runtime/secrets test helpers after mTLS implementation is complete
func generateTestCertificates(t *testing.T) (caCert, clientCert, clientKey []byte) {
	t.Helper()

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate CA private key: %v", err)
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test CA"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             fixedNow,
		NotAfter:              fixedNow.Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Fatalf("Failed to create CA certificate: %v", err)
	}

	clientPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate client private key: %v", err)
	}

	clientTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization:  []string{"Test Client"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:     []string{"localhost"},
		NotBefore:    fixedNow,
		NotAfter:     fixedNow.Add(365 * 24 * time.Hour),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, &clientTemplate, &caTemplate, &clientPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Fatalf("Failed to create client certificate: %v", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertDER,
	})

	clientCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertDER,
	})

	clientKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(clientPrivKey),
	})

	return caCertPEM, clientCertPEM, clientKeyPEM
}

// getNotifierTLSConfig extracts TLSConfig from postMessage-based notifiers for testing
func getNotifierTLSConfig(n notifier.Interface) *tls.Config {
	switch v := n.(type) {
	case *notifier.Forwarder:
		return v.TLSConfig
	case *notifier.Slack:
		return v.TLSConfig
	case *notifier.Alertmanager:
		return v.TLSConfig
	case *notifier.Grafana:
		return v.TLSConfig
	case *notifier.Matrix:
		return v.TLSConfig
	case *notifier.Opsgenie:
		return v.TLSConfig
	case *notifier.PagerDuty:
		return v.TLSConfig
	case *notifier.Rocket:
		return v.TLSConfig
	case *notifier.MSTeams:
		return v.TLSConfig
	case *notifier.Webex:
		return v.TLSConfig
	default:
		return nil
	}
}
