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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
)

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

			err := eventServer.dispatchNotification(context.TODO(), testEvent, alert)
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
		name               string
		alertNamespace     string
		alertSummary       string
		alertEventMetadata map[string]string
		providerNamespace  string
		providerSuspended  bool
		secretNamespace    string
		noCrossNSRefs      bool
		eventMetadata      map[string]string
		wantErr            bool
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
			if tt.providerNamespace != "" {
				provider.Namespace = tt.providerNamespace
			}
			provider.Spec.Suspend = tt.providerSuspended
			if tt.secretNamespace != "" {
				secret.Namespace = tt.secretNamespace
			}
			if tt.eventMetadata != nil {
				event.Metadata = tt.eventMetadata
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

			_, n, _, _, err := eventServer.getNotificationParams(context.TODO(), event, alert)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if tt.alertSummary != "" {
				g.Expect(n.Metadata["summary"]).To(Equal(tt.alertSummary))
			}
			// NOTE: This is performing simple check. Thorough test for event
			// metadata is performed in TestCombineEventMetadata.
			if tt.alertEventMetadata != nil {
				for k, v := range tt.alertEventMetadata {
					g.Expect(n.Metadata).To(HaveKeyWithValue(k, v))
				}
			}
		})
	}
}

func TestCreateNotifier(t *testing.T) {
	secretName := "foo-secret"
	certSecretName := "cert-secret"
	tests := []struct {
		name           string
		providerSpec   *apiv1beta3.ProviderSpec
		secretType     corev1.SecretType
		secretData     map[string][]byte
		certSecretData map[string][]byte
		wantErr        bool
	}{
		{
			name: "no address, no secret ref",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type: "slack",
			},
			wantErr: true,
		},
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
			},
		},
		{
			name: "cert secret reference in caFile with valid CA",
			providerSpec: &apiv1beta3.ProviderSpec{
				Type:          "slack",
				Address:       "https://example.com",
				CertSecretRef: &meta.LocalObjectReference{Name: certSecretName},
			},
			certSecretData: map[string][]byte{
				// Based on https://pkg.go.dev/crypto/tls#X509KeyPair example.
				"caFile": []byte(`-----BEGIN CERTIFICATE-----
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
			},
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
			provider := apiv1beta3.Provider{Spec: *tt.providerSpec}

			_, _, err := createNotifier(context.TODO(), builder.Build(), provider)
			g.Expect(err != nil).To(Equal(tt.wantErr))
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
