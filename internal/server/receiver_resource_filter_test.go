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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestValidateCELExpressionValidExpressions(t *testing.T) {
	validationTests := []string{
		"true",
		"false",
		"req.value == 'test'",
	}

	for _, tt := range validationTests {
		t.Run(tt, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(ValidateResourceFilter(tt)).To(Succeed())
		})
	}
}

func TestValidateCELExpressionInvalidExpressions(t *testing.T) {
	validationTests := []struct {
		expression string
		wantError  string
	}{
		{
			"'test'",
			"CEL expression output type mismatch: expected bool, got string",
		},
		{
			"requrest.body.value",
			"undeclared reference to 'requrest'",
		},
	}

	for _, tt := range validationTests {
		t.Run(tt.expression, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(ValidateResourceFilter(tt.expression)).To(MatchError(ContainSubstring(tt.wantError)))
		})
	}
}

func TestCELEvaluation(t *testing.T) {
	evaluationTests := []struct {
		expression string
		request    *http.Request
		resource   client.Object
		wantResult bool
	}{
		{
			expression: `res.metadata.name == 'test-resource' && req.target.repository == 'hello-world'`,
			request: testNewHTTPRequest(t, http.MethodPost, "/test", map[string]any{
				"target": map[string]any{
					"repository": "hello-world",
				},
			}),
			resource: &apiv1.Receiver{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv1.ReceiverKind,
					APIVersion: apiv1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-resource",
				},
			},
			wantResult: true,
		},
		{
			expression: `req.bool == true`,
			request: testNewHTTPRequest(t, http.MethodPost, "/test", map[string]any{
				"bool": true,
			}),
			resource: &apiv1.Receiver{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv1.ReceiverKind,
					APIVersion: apiv1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-resource",
				},
			},
			wantResult: true,
		},
		{
			expression: `res.metadata.name == 'test-resource' && req.image.source.split(':').last().value().startsWith('v')`,
			request: testNewHTTPRequest(t, http.MethodPost, "/test", map[string]any{
				"image": map[string]any{
					"source": "hello-world:v1.0.0",
				},
			}),
			resource: &apiv1.Receiver{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv1.ReceiverKind,
					APIVersion: apiv1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-resource",
				},
			},
			wantResult: true,
		},
	}

	for _, tt := range evaluationTests {
		t.Run(tt.expression, func(t *testing.T) {
			g := NewWithT(t)
			resourceFilter, err := newResourceFilter(tt.expression, tt.request)
			g.Expect(err).To(Succeed())

			result, err := resourceFilter(context.Background(), tt.resource)
			g.Expect(err).To(Succeed())
			g.Expect(result).To(Equal(&tt.wantResult))
		})
	}
}

func testNewHTTPRequest(t *testing.T, method, target string, body map[string]any) *http.Request {
	var httpBody io.Reader
	g := NewWithT(t)
	if body != nil {
		b, err := json.Marshal(body)
		g.Expect(err).To(Succeed())
		httpBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, target, httpBody)
	g.Expect(err).To(Succeed())

	if httpBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req

}
