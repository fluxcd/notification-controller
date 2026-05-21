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

package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/logger"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

// testOIDCIssuer is a minimal OIDC provider for unit tests: it exposes a
// discovery document and a JWKS, and signs tokens with an in-memory RSA key.
type testOIDCIssuer struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	kid    string
}

func newTestOIDCIssuer(t *testing.T) *testOIDCIssuer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	iss := &testOIDCIssuer{key: key, kid: "test-key"}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	iss.server = server

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                server.URL,
			"jwks_uri":                              server.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
			// The remaining fields are unused by go-oidc but required by the spec.
			"authorization_endpoint":   server.URL + "/auth",
			"response_types_supported": []string{"id_token"},
			"subject_types_supported":  []string{"public"},
		})
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"use": "sig",
					"alg": "RS256",
					"kid": iss.kid,
					"n":   n,
					"e":   e,
				},
			},
		})
	})

	t.Cleanup(server.Close)
	return iss
}

func (iss *testOIDCIssuer) issuerURL() string { return iss.server.URL }

func (iss *testOIDCIssuer) sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = iss.kid
	s, err := tok.SignedString(iss.key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func newOIDCReceiver(name string, providers []apiv1.OIDCProvider) *apiv1.Receiver {
	return &apiv1.Receiver{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiv1.ReceiverSpec{
			Type:          apiv1.GenericOIDCReceiver,
			OIDCProviders: providers,
			Resources: []apiv1.CrossNamespaceObjectReference{{
				APIVersion: apiv1.GroupVersion.String(),
				Kind:       apiv1.ReceiverKind,
				Name:       "target",
			}},
		},
		Status: apiv1.ReceiverStatus{
			WebhookPath: apiv1.ReceiverWebhookPath,
			Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
		},
	}
}

func TestValidateGenericOIDC(t *testing.T) {
	iss := newTestOIDCIssuer(t)
	otherIss := newTestOIDCIssuer(t)

	scheme := runtime.NewScheme()
	_ = apiv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	target := &apiv1.Receiver{
		TypeMeta:   metav1.TypeMeta{Kind: apiv1.ReceiverKind, APIVersion: apiv1.GroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Name: "target"},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "token"},
		Data:       map[string][]byte{"token": []byte("token")},
	}

	baseProvider := apiv1.OIDCProvider{
		IssuerURL: iss.issuerURL(),
		Audience:  "flux",
		Validations: []apiv1.OIDCValidation{
			{Expression: `claims.repository == 'my-org/my-repo'`, Message: "wrong repo"},
			{Expression: `claims.environment == 'production'`, Message: "wrong environment"},
		},
	}

	validClaims := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss":         iss.issuerURL(),
			"aud":         "flux",
			"sub":         "subject",
			"exp":         time.Now().Add(5 * time.Minute).Unix(),
			"iat":         time.Now().Unix(),
			"repository":  "my-org/my-repo",
			"environment": "production",
		}
	}

	tests := []struct {
		name        string
		receiver    *apiv1.Receiver
		bearer      string
		expectError bool
		errContains string
	}{
		{
			name:     "valid token passes validations",
			receiver: newOIDCReceiver("ok", []apiv1.OIDCProvider{baseProvider}),
			bearer:   "Bearer " + iss.sign(t, validClaims()),
		},
		{
			name:        "missing authorization header",
			receiver:    newOIDCReceiver("noauth", []apiv1.OIDCProvider{baseProvider}),
			bearer:      "",
			expectError: true,
			errContains: "Authorization header is missing",
		},
		{
			name:        "malformed bearer",
			receiver:    newOIDCReceiver("malformed", []apiv1.OIDCProvider{baseProvider}),
			bearer:      "Bearer not-a-jwt",
			expectError: true,
			errContains: "parse bearer token",
		},
		{
			name:     "unknown issuer",
			receiver: newOIDCReceiver("unknown", []apiv1.OIDCProvider{baseProvider}),
			bearer: "Bearer " + otherIss.sign(t, jwt.MapClaims{
				"iss": otherIss.issuerURL(),
				"aud": "flux",
				"sub": "s",
				"exp": time.Now().Add(time.Minute).Unix(),
			}),
			expectError: true,
			errContains: "no oidcProvider configured",
		},
		{
			name:     "wrong audience",
			receiver: newOIDCReceiver("aud", []apiv1.OIDCProvider{baseProvider}),
			bearer: "Bearer " + iss.sign(t, jwt.MapClaims{
				"iss": iss.issuerURL(),
				"aud": "someone-else",
				"sub": "s",
				"exp": time.Now().Add(time.Minute).Unix(),
			}),
			expectError: true,
			errContains: "verify OIDC token",
		},
		{
			name:     "expired token",
			receiver: newOIDCReceiver("expired", []apiv1.OIDCProvider{baseProvider}),
			bearer: "Bearer " + iss.sign(t, jwt.MapClaims{
				"iss": iss.issuerURL(),
				"aud": "flux",
				"sub": "s",
				"exp": time.Now().Add(-time.Minute).Unix(),
			}),
			expectError: true,
			errContains: "verify OIDC token",
		},
		{
			name:     "failed validations are aggregated",
			receiver: newOIDCReceiver("bad-claims", []apiv1.OIDCProvider{baseProvider}),
			bearer: "Bearer " + iss.sign(t, jwt.MapClaims{
				"iss":         iss.issuerURL(),
				"aud":         "flux",
				"sub":         "s",
				"exp":         time.Now().Add(time.Minute).Unix(),
				"repository":  "evil/repo",
				"environment": "staging",
			}),
			expectError: true,
			errContains: "wrong repo; wrong environment",
		},
		{
			name: "variables resolve before validations",
			receiver: newOIDCReceiver("vars", []apiv1.OIDCProvider{{
				IssuerURL: iss.issuerURL(),
				Audience:  "flux",
				Variables: []apiv1.OIDCVariable{
					{Name: "allowed", Expression: `['my-org/my-repo']`},
				},
				Validations: []apiv1.OIDCValidation{
					{Expression: `claims.repository in vars.allowed`, Message: "repo not allowed"},
				},
			}}),
			bearer: "Bearer " + iss.sign(t, validClaims()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.receiver, target, secret).
				WithIndex(&apiv1.Receiver{}, WebhookPathIndexKey, IndexReceiverWebhookPath).
				Build()

			s := &ReceiverServer{
				logger:     logger.NewLogger(logger.Options{}),
				kubeClient: fakeClient,
			}

			req := httptest.NewRequest(http.MethodPost, "/hook/", nil)
			if tt.bearer != "" {
				req.Header.Set("Authorization", tt.bearer)
			}

			err := s.validateGenericOIDC(context.Background(), *tt.receiver, req)
			if tt.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
				if tt.errContains != "" {
					g.Expect(err.Error()).To(gomega.ContainSubstring(tt.errContains))
				}
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred(), "unexpected error: %v", err)
			}
		})
	}
}

func TestUnverifiedTokenIssuer(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	payload, _ := json.Marshal(map[string]string{"iss": "https://example.com"})
	tok := fmt.Sprintf("header.%s.sig",
		base64.RawURLEncoding.EncodeToString(payload))

	got, err := unverifiedTokenIssuer(tok)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(got).To(gomega.Equal("https://example.com"))

	_, err = unverifiedTokenIssuer("not-a-jwt")
	g.Expect(err).To(gomega.HaveOccurred())
}
