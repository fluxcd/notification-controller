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
		"request.body.value == 'test'",
	}

	for _, tt := range validationTests {
		t.Run(tt, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(ValidateCELExpression(tt)).To(Succeed())
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
			"invalid expression output type string",
		},
		{
			"requrest.body.value",
			"undeclared reference to 'requrest'",
		},
	}

	for _, tt := range validationTests {
		t.Run(tt.expression, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(ValidateCELExpression(tt.expression)).To(MatchError(ContainSubstring(tt.wantError)))
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
			expression: `resource.metadata.name == 'test-resource' && request.body.target.repository == 'hello-world'`,
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
			expression: `resource.metadata.name == 'test-resource' && request.body.image.source.split(':').last().startsWith('v')`,
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
			evaluator, err := newCELEvaluator(tt.expression, tt.request)
			g.Expect(err).To(Succeed())

			result, err := evaluator(context.Background(), tt.resource)
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
