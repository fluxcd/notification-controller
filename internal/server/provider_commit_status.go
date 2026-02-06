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
	"fmt"
	"slices"
	"strings"

	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/runtime/cel"

	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
)

// newCommitStatusExpression creates a new CEL expression for the commit status ID.
func newCommitStatusExpression(s string) (*cel.Expression, error) {
	return cel.NewExpression(s,
		cel.WithCompile(),
		cel.WithOutputType(types.StringType),
		cel.WithStructVariables("event", "alert", "provider"))
}

// generateDefaultCommitStatus returns a unique string per cluster based on the Provider UID,
// involved object kind and name.
func generateDefaultCommitStatus(providerUID string, event eventv1.Event) string {
	uidParts := strings.Split(providerUID, "-")
	id := fmt.Sprintf("%s/%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name, uidParts[0])
	return strings.ToLower(id)
}

// newCommitStatus evaluates the commit status expression.
func newCommitStatus(ctx context.Context, expr string, notification *eventv1.Event, alert *apiv1beta3.Alert, provider *apiv1beta3.Provider) (string, error) {
	celExpr, err := newCommitStatusExpression(expr)
	if err != nil {
		return "", fmt.Errorf("failed to compile expression: %w", err)
	}

	var (
		eventMap    map[string]any
		providerMap map[string]any
		alertMap    map[string]any
	)

	eventMap, err = runtime.DefaultUnstructuredConverter.ToUnstructured(notification)
	if err != nil {
		return "", fmt.Errorf("failed to convert event to map: %w", err)
	}

	providerMap, err = runtime.DefaultUnstructuredConverter.ToUnstructured(provider)
	if err != nil {
		return "", fmt.Errorf("failed to convert provider object to map: %w", err)
	}

	alertMap, err = runtime.DefaultUnstructuredConverter.ToUnstructured(alert)
	if err != nil {
		return "", fmt.Errorf("failed to convert alert object to map: %w", err)
	}

	vars := map[string]any{
		"event":    eventMap,
		"provider": providerMap,
		"alert":    alertMap,
	}

	result, err := celExpr.EvaluateString(ctx, vars)
	if err != nil {
		return result, err
	}

	return result, nil
}

// isCommitStatusProvider returns true if the provider type is a Git provider.
func isCommitStatusProvider(providerType string) bool {
	gitProviderTypes := []string{
		apiv1beta3.GitHubProvider,
		apiv1beta3.GitLabProvider,
		apiv1beta3.GiteaProvider,
		apiv1beta3.BitbucketServerProvider,
		apiv1beta3.BitbucketProvider,
		apiv1beta3.AzureDevOpsProvider,
	}

	return slices.Contains(gitProviderTypes, providerType)
}

// isCommitStatusUpdate returns true if the event is a commit status update.
func isCommitStatusUpdate(event *eventv1.Event) bool {
	key := event.InvolvedObject.GetObjectKind().GroupVersionKind().Group + "/" + eventv1.MetaCommitStatusKey
	return event.Metadata[key] == eventv1.MetaCommitStatusUpdateValue
}

// hasCommitKey returns true if the event has the commit metadata key.
func hasCommitKey(event *eventv1.Event) bool {
	return event.Metadata[eventv1.Group+"/"+eventv1.MetaCommitKey] != ""
}
