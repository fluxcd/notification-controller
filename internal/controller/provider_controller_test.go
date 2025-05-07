/*
Copyright 2023 The Flux authors

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

package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/fluxcd/pkg/runtime/patch"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
)

func TestProviderReconciler(t *testing.T) {
	g := NewWithT(t)

	timeout := 10 * time.Second

	testns, err := testEnv.CreateNamespace(ctx, "provider-test")
	g.Expect(err).ToNot(HaveOccurred())

	t.Cleanup(func() {
		g.Expect(testEnv.Cleanup(ctx, testns)).ToNot(HaveOccurred())
	})

	provider := &apiv1beta3.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("provider-%s", randStringRunes(5)),
			Namespace: testns.Name,
		},
	}
	providerKey := client.ObjectKeyFromObject(provider)

	// Create without finalizer.
	provider.Spec = apiv1beta3.ProviderSpec{
		Type: "generic",
	}
	g.Expect(testEnv.Create(ctx, provider)).ToNot(HaveOccurred())

	// Should eventually have finalizer.
	g.Eventually(func() bool {
		_ = testEnv.Get(ctx, providerKey, provider)
		return controllerutil.ContainsFinalizer(provider, apiv1.NotificationFinalizer)
	}, timeout, time.Second).Should(BeTrue())

	// Remove finalizer.
	patchHelper, err := patch.NewHelper(provider, testEnv.Client)
	g.Expect(err).ToNot(HaveOccurred())
	controllerutil.RemoveFinalizer(provider, apiv1.NotificationFinalizer)
	g.Expect(patchHelper.Patch(ctx, provider)).ToNot(HaveOccurred())

	// Should eventually have finalizer again.
	g.Eventually(func() bool {
		_ = testEnv.Get(ctx, providerKey, provider)
		return controllerutil.ContainsFinalizer(provider, apiv1.NotificationFinalizer)
	}, timeout, time.Second).Should(BeTrue())

	// Delete the object and verify.
	g.Expect(testEnv.Delete(ctx, provider)).ToNot(HaveOccurred())
	g.Eventually(func() bool {
		if err := testEnv.Get(ctx, providerKey, provider); err != nil {
			return apierrors.IsNotFound(err)
		}
		return false
	}, timeout).Should(BeTrue())
}

func TestProviderReconciler_APIServerValidation(t *testing.T) {
	tests := []struct {
		name             string
		providerType     string
		commitStatusExpr string
		err              string
	}{
		{
			name:             "github provider types can create providers with commitStatusExpr",
			providerType:     "github",
			commitStatusExpr: "event.metadata.namespace + '/' + event.metadata.name + '/' + provider.metadata.uid",
		},
		{
			name:             "gitlab provider types can create providers with commitStatusExpr",
			providerType:     "gitlab",
			commitStatusExpr: "event.metadata.namespace + '/' + event.metadata.name + '/' + provider.metadata.uid",
		},
		{
			name:             "gitea provider types can create providers with commitStatusExpr",
			providerType:     "gitea",
			commitStatusExpr: "event.metadata.namespace + '/' + event.metadata.name + '/' + provider.metadata.uid",
		},
		{
			name:             "bitbucketserver provider types can create providers with commitStatusExpr",
			providerType:     "bitbucketserver",
			commitStatusExpr: "event.metadata.namespace + '/' + event.metadata.name + '/' + provider.metadata.uid",
		},
		{
			name:             "bitbucket provider types can create providers with commitStatusExpr",
			providerType:     "bitbucket",
			commitStatusExpr: "event.metadata.namespace + '/' + event.metadata.name + '/' + provider.metadata.uid",
		},
		{
			name:             "azuredevops provider types can create providers with commitStatusExpr",
			providerType:     "azuredevops",
			commitStatusExpr: "event.metadata.namespace + '/' + event.metadata.name + '/' + provider.metadata.uid",
		},
		{
			name:             "unsupported provider types cannot create providers with commitStatusExpr",
			providerType:     "slack",
			commitStatusExpr: "event.metadata.namespace + '/' + event.metadata.name + '/' + provider.metadata.uid",
			err:              "spec.commitStatusExpr is only supported for the 'github', 'gitlab', 'gitea', 'bitbucketserver', 'bitbucket', 'azuredevops' provider types",
		},
		{
			name:             "github provider types can create providers without commitStatusExpr",
			providerType:     "github",
			commitStatusExpr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := &apiv1beta3.Provider{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "provider-reconcile-",
					Namespace:    "default",
				},
				Spec: apiv1beta3.ProviderSpec{
					Type:             tt.providerType,
					CommitStatusExpr: tt.commitStatusExpr,
				},
			}

			err := testEnv.Create(ctx, obj)
			if err == nil {
				defer func() {
					err := testEnv.Delete(ctx, obj)
					g.Expect(err).ToNot(HaveOccurred())
				}()
			}

			if tt.err != "" {
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
