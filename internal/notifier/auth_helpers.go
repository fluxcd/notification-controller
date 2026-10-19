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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/cache"

	"github.com/fluxcd/notification-controller/api/v1beta3"
)

// newAuthOptions resolves the ServiceAccount used for object-level workload
// identity and builds the authentication options for the given service
// provider. It returns options without a ServiceAccount when neither
// .spec.serviceAccountName nor the default ServiceAccount is set, which makes
// the caller fall back to controller-level authentication.
func newAuthOptions(ctx context.Context, serviceProvider, serviceAccountName,
	defaultServiceAccount string, objectLevelWorkloadIdentity bool,
	providerName, providerNamespace string, tokenClient client.Client,
	tokenCache *cache.TokenCache) ([]auth.Option, error) {

	saName := serviceAccountName
	if saName == "" {
		saName = defaultServiceAccount
	}

	var sa *corev1.ServiceAccount
	if saName != "" {
		if !objectLevelWorkloadIdentity {
			return nil, auth.ErrObjectLevelWorkloadIdentityNotEnabled
		}
		sa = &corev1.ServiceAccount{}
		saRef := client.ObjectKey{Namespace: providerNamespace, Name: saName}
		if err := tokenClient.Get(ctx, saRef, sa); err != nil {
			return nil, fmt.Errorf("failed to get service account '%s': %w", saRef, err)
		}
	}

	buildOpts := []cryptov1.Option{
		cryptov1.WithServiceProvider(serviceProvider),
	}
	if sa != nil {
		buildOpts = append(buildOpts, cryptov1.WithAnnotations(sa.Annotations))
	}
	spec, err := cryptov1.BuildSpec(buildOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to validate configuration: %w", err)
	}

	provider := &v1beta3.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: providerNamespace,
		},
	}
	opts := []auth.Option{
		auth.WithClient(tokenClient),
		auth.WithObject(v1beta3.GroupVersion.Group, v1beta3.ProviderKind, provider),
	}
	opts = append(opts, auth.OptionsFromSpec(spec.Auth)...)
	if sa != nil {
		opts = append(opts, auth.WithServiceAccountName(sa.Name))
	}
	if tokenCache != nil {
		opts = append(opts,
			auth.WithCache(*tokenCache),
			auth.WithCacheOperation(OperationPost))
	}
	return opts, nil
}
