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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

// validateGenericOIDC verifies the bearer token of an incoming request against
// one of the configured OIDC providers and evaluates the provider's CEL
// variables and validations over the verified claims.
func (s *ReceiverServer) validateGenericOIDC(ctx context.Context, receiver apiv1.Receiver, r *http.Request) error {
	if len(receiver.Spec.OIDCProviders) == 0 {
		return fmt.Errorf("generic-oidc receiver has no oidcProviders configured")
	}

	bearer := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(bearer, prefix) {
		return fmt.Errorf("the Authorization header is missing or malformed")
	}
	rawToken := strings.TrimSpace(bearer[len(prefix):])
	if rawToken == "" {
		return fmt.Errorf("the Authorization header is missing a bearer token")
	}

	iss, err := unverifiedTokenIssuer(rawToken)
	if err != nil {
		return fmt.Errorf("failed to parse bearer token: %w", err)
	}

	var provider *apiv1.OIDCProvider
	for i := range receiver.Spec.OIDCProviders {
		if receiver.Spec.OIDCProviders[i].IssuerURL == iss {
			provider = &receiver.Spec.OIDCProviders[i]
			break
		}
	}
	if provider == nil {
		return fmt.Errorf("no oidcProvider configured for issuer %q", iss)
	}

	oidcProvider, err := oidc.NewProvider(ctx, provider.IssuerURL)
	if err != nil {
		return fmt.Errorf("failed to initialize OIDC provider for issuer %q: %w", provider.IssuerURL, err)
	}
	verifier := oidcProvider.Verifier(&oidc.Config{ClientID: provider.Audience})

	idToken, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return fmt.Errorf("failed to verify OIDC token: %w", err)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return fmt.Errorf("failed to extract OIDC token claims: %w", err)
	}

	processor, err := newOIDCClaimsProcessor(*provider)
	if err != nil {
		return fmt.Errorf("invalid oidcProvider configuration for issuer %q: %w", provider.IssuerURL, err)
	}
	if err := processor.Evaluate(ctx, claims); err != nil {
		return err
	}

	return nil
}

// unverifiedTokenIssuer extracts the 'iss' claim from a JWT without verifying
// its signature. The returned value is only used to look up the configured
// OIDC provider; the token is verified before any claim is trusted.
func unverifiedTokenIssuer(rawToken string) (string, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode token payload: %w", err)
	}
	var c struct {
		Iss string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return "", fmt.Errorf("failed to parse token payload: %w", err)
	}
	if c.Iss == "" {
		return "", fmt.Errorf("token is missing the 'iss' claim")
	}
	return c.Iss, nil
}
