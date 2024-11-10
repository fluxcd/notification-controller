/*
Copyright 2024 The Flux authors

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

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	celext "github.com/google/cel-go/ext"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateCELEXpression accepts a CEL expression and will parse and check that
// it's valid, if it's not valid an error is returned.
func ValidateCELExpression(s string) error {
	_, err := newCELProgram(s)
	return err
}

func newCELProgram(expr string) (cel.Program, error) {
	env, err := makeCELEnv()
	if err != nil {
		return nil, err
	}
	parsed, issues := env.Parse(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to parse expression %v: %w", expr, issues.Err())
	}

	checked, issues := env.Check(parsed)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("expression %v check failed: %w", expr, issues.Err())
	}

	prg, err := env.Program(checked, cel.EvalOptions(cel.OptOptimize), cel.InterruptCheckFrequency(100))
	if err != nil {
		return nil, fmt.Errorf("expression %v failed to create a Program: %w", expr, err)
	}

	return prg, nil
}

func newCELEvaluator(expr string, req *http.Request) (resourcePredicate, error) {
	prg, err := newCELProgram(expr)
	if err != nil {
		return nil, err
	}

	body := map[string]any{}
	// Only decodes the body for the expression if the body is JSON.
	// Technically you could generate several resources without any body.
	if isJSONContent(req) {
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("failed to parse request body as JSON: %s", err)
		}
	}

	return func(ctx context.Context, obj client.Object) (*bool, error) {
		data, err := clientObjectToMap(obj)
		if err != nil {
			return nil, err
		}

		out, _, err := prg.ContextEval(ctx, map[string]any{
			"resource": data,
			"request": map[string]any{
				"body": body,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("expression %v failed to evaluate: %w", expr, err)
		}

		v, ok := out.(types.Bool)
		if !ok {
			return nil, fmt.Errorf("expression %q did not return a boolean value", expr)
		}

		result := v.Value().(bool)

		return &result, nil
	}, nil
}

func makeCELEnv() (*cel.Env, error) {
	mapStrDyn := decls.NewMapType(decls.String, decls.Dyn)
	return cel.NewEnv(
		celext.Strings(),
		notifications(),
		cel.Declarations(
			decls.NewVar("resource", mapStrDyn),
			decls.NewVar("request", mapStrDyn),
		))
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

func notifications() cel.EnvOption {
	r, err := types.NewRegistry()
	if err != nil {
		panic(err) // TODO: Do something better?
	}

	return cel.Lib(&notificationsLib{registry: r})
}

type notificationsLib struct {
	registry *types.Registry
}

// LibraryName implements the SingletonLibrary interface method.
func (*notificationsLib) LibraryName() string {
	return "flux.notifications.lib"
}

// CompileOptions implements the Library interface method.
func (l *notificationsLib) CompileOptions() []cel.EnvOption {
	listStrDyn := cel.ListType(cel.DynType)
	opts := []cel.EnvOption{
		cel.Function("first",
			cel.MemberOverload("first_list", []*cel.Type{listStrDyn}, cel.DynType,
				cel.UnaryBinding(listFirst))),
		cel.Function("last",
			cel.MemberOverload("last_list", []*cel.Type{listStrDyn}, cel.DynType,
				cel.UnaryBinding(listLast))),
	}

	return opts
}

// ProgramOptions implements the Library interface method.
func (*notificationsLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func listLast(val ref.Val) ref.Val {
	l := val.(traits.Lister)
	sz := l.Size().Value().(int64)

	if sz == 0 {
		return types.NullValue
	}

	return l.Get(types.Int(sz - 1))
}

func listFirst(val ref.Val) ref.Val {
	l := val.(traits.Lister)
	sz := l.Size().Value().(int64)

	if sz == 0 {
		return types.NullValue
	}

	return l.Get(types.Int(0))
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
