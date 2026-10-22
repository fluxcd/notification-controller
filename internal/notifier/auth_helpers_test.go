/*
Copyright 2026 The Flux authors

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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/cache"
)

func TestNewAuthOptions(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-sa",
			Namespace: "apps",
		},
	}
	newClient := func(objects ...client.Object) client.Client {
		return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	}

	t.Run("no service account uses controller-level authentication", func(t *testing.T) {
		g := NewWithT(t)
		opts, err := newAuthOptions(context.Background(), cryptov1.ProviderAzure,
			"", "", false, "provider", "apps", newClient(), nil)
		g.Expect(err).NotTo(HaveOccurred())

		var o auth.Options
		o.Apply(opts...)
		g.Expect(o.ServiceAccountName).To(BeEmpty())
		g.Expect(o.ObjectReference.Namespace).To(Equal("apps"))
	})

	t.Run("service account requires the feature gate", func(t *testing.T) {
		g := NewWithT(t)
		_, err := newAuthOptions(context.Background(), cryptov1.ProviderAzure,
			"auth-sa", "", false, "provider", "apps", newClient(sa), nil)
		g.Expect(err).To(MatchError(auth.ErrObjectLevelWorkloadIdentityNotEnabled))
	})

	t.Run("resolves the explicit service account", func(t *testing.T) {
		g := NewWithT(t)
		opts, err := newAuthOptions(context.Background(), cryptov1.ProviderAzure,
			"auth-sa", "", true, "provider", "apps", newClient(sa), nil)
		g.Expect(err).NotTo(HaveOccurred())

		var o auth.Options
		o.Apply(opts...)
		g.Expect(o.ServiceAccountName).To(Equal("auth-sa"))
		g.Expect(o.ObjectReference.Name).To(Equal("provider"))
		g.Expect(o.ObjectReference.Namespace).To(Equal("apps"))
	})

	t.Run("resolves the default service account", func(t *testing.T) {
		g := NewWithT(t)
		opts, err := newAuthOptions(context.Background(), cryptov1.ProviderGCP,
			"", "auth-sa", true, "provider", "apps", newClient(sa), nil)
		g.Expect(err).NotTo(HaveOccurred())

		var o auth.Options
		o.Apply(opts...)
		g.Expect(o.ServiceAccountName).To(Equal("auth-sa"))
	})

	t.Run("missing service account", func(t *testing.T) {
		g := NewWithT(t)
		_, err := newAuthOptions(context.Background(), cryptov1.ProviderGCP,
			"missing", "", true, "provider", "apps", newClient(), nil)
		g.Expect(err).To(MatchError(ContainSubstring("failed to get service account 'apps/missing'")))
	})

	t.Run("invalid service account annotations", func(t *testing.T) {
		g := NewWithT(t)
		invalidSA := sa.DeepCopy()
		invalidSA.Annotations = map[string]string{
			cryptov1.AnnotationCredentialProvider: cryptov1.CredentialProviderKubernetes,
		}
		_, err := newAuthOptions(context.Background(), cryptov1.ProviderAzure,
			"auth-sa", "", true, "provider", "apps", newClient(invalidSA), nil)
		g.Expect(err).To(MatchError(ContainSubstring("must be set together")))
	})
}

func TestNewAuthOptionsCache(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-sa",
			Namespace: "apps",
		},
	}
	tokenCache, err := cache.NewTokenCache(10)
	g.Expect(err).NotTo(HaveOccurred())

	opts, err := newAuthOptions(context.Background(), cryptov1.ProviderAzure,
		"auth-sa", "", true, "provider", "apps",
		fake.NewClientBuilder().WithScheme(scheme).WithObjects(sa).Build(), tokenCache)
	g.Expect(err).NotTo(HaveOccurred())

	var o auth.Options
	o.Apply(opts...)
	g.Expect(o.Cache).To(Equal(tokenCache))
	g.Expect(o.CacheOperation).To(Equal(OperationPost))
}
