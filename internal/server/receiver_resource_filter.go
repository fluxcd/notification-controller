/*
Copyright 2025 The Flux authors

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
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/fluxcd/pkg/runtime/cel"
	"github.com/google/cel-go/common/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

type resourceFilter func(context.Context, client.Object) (*bool, error)

// ResourceFilterOption configures the CEL environment used to compile and
// evaluate a Receiver resource filter expression.
type ResourceFilterOption func(*resourceFilterOptions)

type resourceFilterOptions struct {
	withClaims bool
}

// WithClaims declares the claims variable in the filter CEL environment. It is
// used by generic-oidc receivers to expose the verified OIDC token claims to
// the expression.
func WithClaims() ResourceFilterOption {
	return func(o *resourceFilterOptions) {
		o.withClaims = true
	}
}

// ValidateResourceFilter accepts a CEL expression and will parse and check that
// it's valid, if it's not valid an error is returned.
func ValidateResourceFilter(s string, opts ...ResourceFilterOption) error {
	_, err := newFilterExpression(s, opts...)
	return err
}

func newFilterExpression(s string, opts ...ResourceFilterOption) (*cel.Expression, error) {
	var o resourceFilterOptions
	for _, opt := range opts {
		opt(&o)
	}

	vars := []string{"res", "req"}
	if o.withClaims {
		vars = append(vars, "claims")
	}

	return cel.NewExpression(s,
		cel.WithCompile(),
		cel.WithOutputType(types.BoolType),
		cel.WithStructVariables(vars...))
}

// resourceFilterEvaluator compiles resource filter CEL expressions that share a
// single parsed webhook request body and, for generic-oidc receivers, the
// verified token claims. The body is read once so the top-level resourceFilter
// and the per-resource filters can all be evaluated against the same request.
type resourceFilterEvaluator struct {
	req    map[string]any
	claims map[string]any
}

// newResourceFilterEvaluator parses the webhook request body once and captures
// the verified OIDC token claims (if any) for later expression evaluation.
func newResourceFilterEvaluator(r *http.Request, result *validationResult) (*resourceFilterEvaluator, error) {
	// Only decodes the body for the expression if the body is JSON.
	// Technically you could generate several resources without any body.
	var req map[string]any
	if !isJSONContent(r) {
		req = map[string]any{}
	} else if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("failed to parse request body as JSON: %s", err)
	}

	var claims map[string]any
	if result != nil {
		claims = result.claims
	}

	return &resourceFilterEvaluator{req: req, claims: claims}, nil
}

// filter compiles a single CEL expression into a resourceFilter that evaluates
// it against each resource. When the evaluator carries OIDC token claims, they
// are exposed as the claims variable.
func (e *resourceFilterEvaluator) filter(expr string) (resourceFilter, error) {
	var opts []ResourceFilterOption
	if e.claims != nil {
		opts = append(opts, WithClaims())
	}

	celExpr, err := newFilterExpression(expr, opts...)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, obj client.Object) (*bool, error) {
		res, err := clientObjectToMap(obj)
		if err != nil {
			return nil, err
		}

		vars := map[string]any{
			"res": res,
			"req": e.req,
		}
		if e.claims != nil {
			vars["claims"] = e.claims
		}

		result, err := celExpr.EvaluateBoolean(ctx, vars)
		if err != nil {
			return nil, err
		}

		return &result, nil
	}, nil
}

// allResourceFilters stacks filters so a resource is accepted only when every
// filter accepts it. Nil filters are ignored; if no filter remains it returns
// nil, leaving the decision to the caller's default.
func allResourceFilters(filters ...resourceFilter) resourceFilter {
	var active []resourceFilter
	for _, f := range filters {
		if f != nil {
			active = append(active, f)
		}
	}
	if len(active) == 0 {
		return nil
	}

	return func(ctx context.Context, obj client.Object) (*bool, error) {
		for _, f := range active {
			accept, err := f(ctx, obj)
			if err != nil {
				return nil, err
			}
			if !*accept {
				return accept, nil
			}
		}
		return new(true), nil
	}
}

// newResourceFilters builds the effective filter for each resource referenced by
// the Receiver. The top-level resourceFilter and any per-resource filter stack:
// a resource is reconciled only when all configured expressions accept it. The
// webhook request body is parsed at most once and shared across expressions.
//
// The returned slice is aligned with receiver.Spec.Resources; a nil element
// means no filter applies and the resource should be accepted.
func newResourceFilters(r *http.Request, receiver apiv1.Receiver, result *validationResult) ([]resourceFilter, error) {
	filters := make([]resourceFilter, len(receiver.Spec.Resources))

	hasFilters := receiver.Spec.ResourceFilter != ""
	for i := range receiver.Spec.Resources {
		if receiver.Spec.Resources[i].Filter != "" {
			hasFilters = true
		}
	}
	if !hasFilters {
		return filters, nil
	}

	evaluator, err := newResourceFilterEvaluator(r, result)
	if err != nil {
		return nil, err
	}

	var topFilter resourceFilter
	if receiver.Spec.ResourceFilter != "" {
		topFilter, err = evaluator.filter(receiver.Spec.ResourceFilter)
		if err != nil {
			return nil, err
		}
	}

	for i := range receiver.Spec.Resources {
		var perResource resourceFilter
		if expr := receiver.Spec.Resources[i].Filter; expr != "" {
			perResource, err = evaluator.filter(expr)
			if err != nil {
				return nil, err
			}
		}
		filters[i] = allResourceFilters(topFilter, perResource)
	}

	return filters, nil
}

func isJSONContent(r *http.Request) bool {
	contentType := r.Header.Get("Content-type")
	for _, v := range strings.Split(contentType, ",") {
		t, _, err := mime.ParseMediaType(v)
		if err != nil {
			break
		}
		if t == "application/json" {
			return true
		}
	}

	return false
}

func clientObjectToMap(v client.Object) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PartialObjectMetadata from resource for CEL expression: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PartialObjectMetadata from resource for CEL expression: %w", err)
	}

	return result, nil
}
