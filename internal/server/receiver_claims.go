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
	"fmt"
	"regexp"
	"strings"

	"github.com/fluxcd/pkg/runtime/cel"
	"github.com/google/cel-go/common/types"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

// oidcIssuerURLPattern mirrors the +kubebuilder:validation:Pattern marker on
// OIDCProvider.IssuerURL and lets the controller reject invalid URLs even on
// clusters whose API server skipped the schema check.
var oidcIssuerURLPattern = regexp.MustCompile(`^https?://`)

// oidcClaimsProcessor evaluates the CEL variables and validations configured
// on an OIDCProvider against a verified set of token claims.
type oidcClaimsProcessor struct {
	variables   []namedExpression
	validations []validationExpression
}

type namedExpression struct {
	name string
	expr *cel.Expression
}

type validationExpression struct {
	expr    *cel.Expression
	message string
}

// newOIDCClaimsProcessor compiles the variable and validation CEL expressions
// configured on the given provider.
func newOIDCClaimsProcessor(p apiv1.OIDCProvider) (*oidcClaimsProcessor, error) {
	cp := &oidcClaimsProcessor{}

	for _, v := range p.Variables {
		e, err := cel.NewExpression(v.Expression,
			cel.WithCompile(),
			cel.WithStructVariables("claims", "vars"))
		if err != nil {
			return nil, fmt.Errorf("invalid expression for variable %q: %w", v.Name, err)
		}
		cp.variables = append(cp.variables, namedExpression{name: v.Name, expr: e})
	}

	for _, v := range p.Validations {
		e, err := cel.NewExpression(v.Expression,
			cel.WithCompile(),
			cel.WithOutputType(types.BoolType),
			cel.WithStructVariables("claims", "vars"))
		if err != nil {
			return nil, fmt.Errorf("invalid expression for validation %q: %w", v.Message, err)
		}
		cp.validations = append(cp.validations, validationExpression{expr: e, message: v.Message})
	}

	return cp, nil
}

// Evaluate runs the variables, then evaluates every validation. If one or more
// validations fail, their messages are aggregated into a single error.
func (cp *oidcClaimsProcessor) Evaluate(ctx context.Context, claims map[string]any) error {
	variables := map[string]any{}
	data := map[string]any{
		"claims": claims,
		"vars":   variables,
	}

	for _, v := range cp.variables {
		val, err := v.expr.Evaluate(ctx, data)
		if err != nil {
			return fmt.Errorf("failed to evaluate variable %q: %w", v.name, err)
		}
		variables[v.name] = val
	}

	var failures []string
	for _, v := range cp.validations {
		ok, err := v.expr.EvaluateBoolean(ctx, data)
		if err != nil {
			return fmt.Errorf("failed to evaluate validation: %w", err)
		}
		if !ok {
			failures = append(failures, v.message)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("token validation failed: %s", strings.Join(failures, "; "))
	}

	return nil
}

// ValidateOIDCProvidersSpec mirrors the CRD schema constraints on
// OIDCProviders (required fields, IssuerURL pattern, uniqueness, validation
// cardinality) for clusters whose API server does not enforce the kubebuilder
// markers. Errors are intended to be surfaced under ValidationFailedReason.
func ValidateOIDCProvidersSpec(providers []apiv1.OIDCProvider) error {
	seen := make(map[string]struct{}, len(providers))
	for i, p := range providers {
		if p.IssuerURL == "" {
			return fmt.Errorf("oidcProviders[%d]: issuerURL is required", i)
		}
		if !oidcIssuerURLPattern.MatchString(p.IssuerURL) {
			return fmt.Errorf("oidcProviders[%d]: issuerURL %q must start with http:// or https://", i, p.IssuerURL)
		}
		if _, dup := seen[p.IssuerURL]; dup {
			return fmt.Errorf("oidcProviders[%d]: duplicate issuerURL %q", i, p.IssuerURL)
		}
		seen[p.IssuerURL] = struct{}{}
		if len(p.Validations) == 0 {
			return fmt.Errorf("oidcProviders[%d] (issuerURL=%q): at least one validation is required; without validations any caller able to obtain a token from this issuer can trigger the Receiver", i, p.IssuerURL)
		}
		for j, v := range p.Variables {
			if v.Name == "" {
				return fmt.Errorf("oidcProviders[%d] (issuerURL=%q): variables[%d]: name is required", i, p.IssuerURL, j)
			}
			if v.Expression == "" {
				return fmt.Errorf("oidcProviders[%d] (issuerURL=%q): variables[%d] (name=%q): expression is required", i, p.IssuerURL, j, v.Name)
			}
		}
		for j, v := range p.Validations {
			if v.Expression == "" {
				return fmt.Errorf("oidcProviders[%d] (issuerURL=%q): validations[%d]: expression is required", i, p.IssuerURL, j)
			}
			if v.Message == "" {
				return fmt.Errorf("oidcProviders[%d] (issuerURL=%q): validations[%d]: message is required", i, p.IssuerURL, j)
			}
		}
	}
	return nil
}

// CompileOIDCProviders compiles every CEL expression configured on every
// provider and returns the first error encountered. Errors are intended to be
// surfaced under InvalidCELExpressionReason. ValidateOIDCProvidersSpec must
// have been called first.
func CompileOIDCProviders(providers []apiv1.OIDCProvider) error {
	for i, p := range providers {
		if _, err := newOIDCClaimsProcessor(p); err != nil {
			return fmt.Errorf("oidcProviders[%d] (issuerURL=%q): %w", i, p.IssuerURL, err)
		}
	}
	return nil
}
