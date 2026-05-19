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
	"testing"

	"github.com/onsi/gomega"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

func TestOIDCClaimsProcessor_Evaluate(t *testing.T) {
	tests := []struct {
		name        string
		provider    apiv1.OIDCProvider
		claims      map[string]any
		expectError bool
		errContains string
	}{
		{
			name: "no validations always passes",
			provider: apiv1.OIDCProvider{
				IssuerURL: "https://issuer",
				Audience:  "id",
			},
			claims: map[string]any{"sub": "x"},
		},
		{
			name: "passing validation",
			provider: apiv1.OIDCProvider{
				IssuerURL: "https://issuer",
				Audience:  "id",
				Validations: []apiv1.OIDCValidation{
					{Expression: `claims.repo == 'org/repo'`, Message: "wrong repo"},
				},
			},
			claims: map[string]any{"repo": "org/repo"},
		},
		{
			name: "all failures aggregated",
			provider: apiv1.OIDCProvider{
				IssuerURL: "https://issuer",
				Audience:  "id",
				Validations: []apiv1.OIDCValidation{
					{Expression: `claims.repo == 'org/repo'`, Message: "wrong repo"},
					{Expression: `claims.env == 'prod'`, Message: "wrong env"},
				},
			},
			claims:      map[string]any{"repo": "evil", "env": "dev"},
			expectError: true,
			errContains: "wrong repo; wrong env",
		},
		{
			name: "variables flow into validations",
			provider: apiv1.OIDCProvider{
				IssuerURL: "https://issuer",
				Audience:  "id",
				Variables: []apiv1.OIDCVariable{
					{Name: "allowed", Expression: `['org/repo']`},
				},
				Validations: []apiv1.OIDCValidation{
					{Expression: `claims.repo in vars.allowed`, Message: "repo denied"},
				},
			},
			claims: map[string]any{"repo": "org/repo"},
		},
		{
			name: "variables can reference earlier variables",
			provider: apiv1.OIDCProvider{
				IssuerURL: "https://issuer",
				Audience:  "id",
				Variables: []apiv1.OIDCVariable{
					{Name: "owner", Expression: `claims.owner`},
					{Name: "allowed", Expression: `[vars.owner + '/repo']`},
				},
				Validations: []apiv1.OIDCValidation{
					{Expression: `claims.repo in vars.allowed`, Message: "repo denied"},
				},
			},
			claims: map[string]any{"owner": "org", "repo": "org/repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			cp, err := newOIDCClaimsProcessor(tt.provider)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			err = cp.Evaluate(context.Background(), tt.claims)
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

func TestValidateOIDCProvidersSpec(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	stubValidation := []apiv1.OIDCValidation{
		{Expression: "claims.repository_owner == 'my-org'", Message: "wrong org"},
	}

	g.Expect(ValidateOIDCProvidersSpec([]apiv1.OIDCProvider{
		{IssuerURL: "https://a", Audience: "aud", Validations: stubValidation},
		{IssuerURL: "https://b", Audience: "aud", Validations: stubValidation},
	})).To(gomega.Succeed())

	err := ValidateOIDCProvidersSpec([]apiv1.OIDCProvider{{Audience: "aud", Validations: stubValidation}})
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("issuerURL is required")))

	err = ValidateOIDCProvidersSpec([]apiv1.OIDCProvider{{IssuerURL: "https://a", Validations: stubValidation}})
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("audience is required")))

	err = ValidateOIDCProvidersSpec([]apiv1.OIDCProvider{{IssuerURL: "example.com", Audience: "aud", Validations: stubValidation}})
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("must start with http:// or https://")))

	err = ValidateOIDCProvidersSpec([]apiv1.OIDCProvider{{
		IssuerURL:   "https://a",
		Audience:    "aud",
		Variables:   []apiv1.OIDCVariable{{Name: "x"}},
		Validations: stubValidation,
	}})
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("variables[0] (name=\"x\"): expression is required")))

	err = ValidateOIDCProvidersSpec([]apiv1.OIDCProvider{{
		IssuerURL:   "https://a",
		Audience:    "aud",
		Validations: []apiv1.OIDCValidation{{Message: "msg"}},
	}})
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("validations[0]: expression is required")))

	err = ValidateOIDCProvidersSpec([]apiv1.OIDCProvider{
		{IssuerURL: "https://a", Audience: "aud", Validations: stubValidation},
		{IssuerURL: "https://a", Audience: "aud", Validations: stubValidation},
	})
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("duplicate issuerURL")))

	err = ValidateOIDCProvidersSpec([]apiv1.OIDCProvider{{IssuerURL: "https://a", Audience: "aud"}})
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("at least one validation is required")))
}

func TestCompileOIDCProviders(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	g.Expect(CompileOIDCProviders([]apiv1.OIDCProvider{{
		IssuerURL:   "https://a",
		Audience:    "aud",
		Validations: []apiv1.OIDCValidation{{Expression: "claims.x == 'y'", Message: "ok"}},
	}})).To(gomega.Succeed())

	err := CompileOIDCProviders([]apiv1.OIDCProvider{{
		IssuerURL:   "https://a",
		Audience:    "aud",
		Validations: []apiv1.OIDCValidation{{Expression: "not-valid-cel ===", Message: "x"}},
	}})
	g.Expect(err).To(gomega.HaveOccurred())
}
