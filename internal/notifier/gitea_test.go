/*
Copyright 2022 The Flux authors

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

package notifier

import (
	"context"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewGiteaBasic(t *testing.T) {
	g, err := NewGitea("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "https://try.gitea.io/foo/bar", "foobar", nil)
	assert.Nil(t, err)
	assert.Equal(t, g.Owner, "foo")
	assert.Equal(t, g.Repo, "bar")
	assert.Equal(t, g.BaseURL, "https://try.gitea.io")
}

func TestNewGiteaInvalidUrl(t *testing.T) {
	_, err := NewGitea("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "https://try.gitea.io/foo/bar/baz", "foobar", nil)
	assert.NotNil(t, err)
}

func TestNewGiteaEmptyToken(t *testing.T) {
	_, err := NewGitea("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "https://try.gitea.io/foo/bar", "", nil)
	assert.NotNil(t, err)
}

func TestGitea_Post(t *testing.T) {
	g, err := NewGitea("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "https://try.gitea.io/foo/bar", "foobar", nil)
	assert.Nil(t, err)

	event := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Kustomization",
			Namespace: "flux-system",
			Name:      "podinfo-repo",
		},
		Severity: "info",
		Timestamp: metav1.Time{
			Time: time.Now(),
		},
		Metadata: map[string]string{
			eventv1.MetaRevisionKey: "main/1234567890",
		},
		Message: "Service/podinfo/podinfo configured",
		Reason:  "",
	}
	err = g.Post(context.Background(), event)
	assert.NotNil(t, err)
	assert.ErrorContains(t, err, "404 Not Found")
}
