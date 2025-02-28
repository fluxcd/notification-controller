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
)

type resourceFilter func(context.Context, client.Object) (*bool, error)

// ValidateResourceFilter accepts a CEL expression and will parse and check that
// it's valid, if it's not valid an error is returned.
func ValidateResourceFilter(s string) error {
	_, err := newFilterExpression(s)
	return err
}

func newFilterExpression(s string) (*cel.Expression, error) {
	return cel.NewExpression(s,
		cel.WithCompile(),
		cel.WithOutputType(types.BoolType),
		cel.WithStructVariables("res", "req"))
}

func newResourceFilter(expr string, r *http.Request) (resourceFilter, error) {
	celExpr, err := newFilterExpression(expr)
	if err != nil {
		return nil, err
	}

	// Only decodes the body for the expression if the body is JSON.
	// Technically you could generate several resources without any body.
	var req map[string]any
	if !isJSONContent(r) {
		req = map[string]any{}
	} else if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("failed to parse request body as JSON: %s", err)
	}

	return func(ctx context.Context, obj client.Object) (*bool, error) {
		res, err := clientObjectToMap(obj)
		if err != nil {
			return nil, err
		}

		result, err := celExpr.EvaluateBoolean(ctx, map[string]any{
			"res": res,
			"req": req,
		})
		if err != nil {
			return nil, err
		}

		return &result, nil
	}, nil
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
