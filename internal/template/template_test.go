package template

import (
	"testing"

	"github.com/fluxcd/pkg/runtime/events"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestBasic(t *testing.T) {
	event := events.Event{
		Reason: "Test reason",
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "foo",
			Name:       "bar",
		},
		Metadata: map[string]string{
			"revision": "abcd123",
		},
	}
	tmplString := "{{ .Reason }} - {{ .InvolvedObject.APIVersion }}/{{ .InvolvedObject.Kind }}/{{ .InvolvedObject.Namespace }}/{{ .InvolvedObject.Name }} - {{ .Metadata.Revision }}"
	result, err := templateString(event, tmplString)
	require.NoError(t, err)
	require.Equal(t, "Test reason - apps/v1/Deployment/foo/bar - abcd123", result)
}
