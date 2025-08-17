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

package notifier

import (
	"context"
	"fmt"
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/cache"

	"github.com/fluxcd/notification-controller/api/v1beta3"
)

// newManagedIdentityToken is used to attempt credential-free authentication.
func newManagedIdentityToken(ctx context.Context, proxy, serviceAccountName, providerName, providerNamespace, scope string, tokenClient client.Client, tokenCache *cache.TokenCache) (string, error) {
	opts := []auth.Option{
		auth.WithScopes(scope),
		auth.WithClient(tokenClient),
		auth.WithServiceAccountNamespace(providerNamespace),
	}

	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return "", fmt.Errorf("error parsing proxy URL: %w", err)
		}
		opts = append(opts, auth.WithProxyURL(*proxyURL))
	}

	if serviceAccountName != "" {
		opts = append(opts, auth.WithServiceAccountName(serviceAccountName))
	}

	if tokenCache != nil {
		involvedObject := cache.InvolvedObject{
			Kind:      v1beta3.ProviderKind,
			Name:      providerName,
			Namespace: providerNamespace,
			Operation: OperationPost,
		}
		opts = append(opts, auth.WithCache(*tokenCache, involvedObject))
	}

	token, err := auth.GetAccessToken(ctx, azure.Provider{}, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	return token.(*azure.Token).Token, nil
}
