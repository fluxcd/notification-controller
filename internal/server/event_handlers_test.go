package server

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/events"

	"github.com/fluxcd/notification-controller/api/v1beta1"
)

func TestFindMatchingAlerts(t *testing.T) {
	event := &events.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Kustomization",
			Name:      "app",
			Namespace: "foo",
		},
		Message: "foobar",
	}
	readyStatus := v1beta1.AlertStatus{
		ObservedGeneration: 1,
		Conditions: []v1.Condition{
			{
				Type:    meta.ReadyCondition,
				Status:  v1.ConditionTrue,
				Reason:  v1beta1.InitializedReason,
				Message: v1beta1.InitializedReason,
			},
		},
	}

	tests := map[string]struct {
		spec        v1beta1.AlertSpec
		expectedLen int
	}{
		"simple": {
			spec: v1beta1.AlertSpec{
				Suspend:       false,
				EventSeverity: events.EventSeverityInfo,
				EventSources: []v1beta1.CrossNamespaceObjectReference{
					{
						Kind:      "Kustomization",
						Name:      "app",
						Namespace: "foo",
					},
				},
			},
			expectedLen: 1,
		},
		"suspended": {
			spec: v1beta1.AlertSpec{
				Suspend:       true,
				EventSeverity: events.EventSeverityInfo,
				EventSources: []v1beta1.CrossNamespaceObjectReference{
					{
						Kind:      "Kustomization",
						Name:      "app",
						Namespace: "foo",
					},
				},
			},
			expectedLen: 0,
		},
		"excluded": {
			spec: v1beta1.AlertSpec{
				Suspend:       false,
				EventSeverity: events.EventSeverityInfo,
				EventSources: []v1beta1.CrossNamespaceObjectReference{
					{
						Kind:      "Kustomization",
						Name:      "app",
						Namespace: "foo",
					},
				},
				ExclusionList: []string{"foobar"},
			},
			expectedLen: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			allAlerts := v1beta1.AlertList{
				Items: []v1beta1.Alert{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      "alert",
							Namespace: "foo",
						},
						Spec:   tc.spec,
						Status: readyStatus,
					},
				},
			}
			alerts := findMatchingAlerts(logr.Discard(), allAlerts, event)
			require.Len(t, alerts, tc.expectedLen)
		})
	}
}

func TestRedact(t *testing.T) {
	tests := map[string]struct {
		input    error
		token    string
		expected error
	}{
		"simple": {
			input:    errors.New("This is a secret error message"),
			token:    "secret",
			expected: fmt.Errorf("This is a %s error message", redactReplacement),
		},
		"multiple": {
			input:    errors.New("This is a secret error message secret"),
			token:    "secret",
			expected: fmt.Errorf("This is a %[1]s error message %[1]s", redactReplacement),
		},
		"part of word": {
			input:    errors.New("This is a secreterror message"),
			token:    "secret",
			expected: fmt.Errorf("This is a %serror message", redactReplacement),
		},
		"no match": {
			input:    errors.New("This is a secret error message"),
			token:    "token",
			expected: errors.New("This is a secret error message"),
		},
		"empty token": {
			input:    errors.New("This is a secret error message"),
			token:    "",
			expected: errors.New("This is a secret error message"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := redact(tc.input, tc.token)
			require.Equal(t, tc.expected, err)
		})
	}
}
