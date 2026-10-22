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

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/cache"
)

// newManagedIdentityToken is used to attempt credential-free authentication.
func newManagedIdentityToken(ctx context.Context, proxy, serviceAccountName, defaultServiceAccount string,
	objectLevelWorkloadIdentity bool, providerName, providerNamespace, scope string,
	tokenClient client.Client, tokenCache *cache.TokenCache) (string, error) {

	opts, err := newAuthOptions(ctx, cryptov1.ProviderAzure, serviceAccountName,
		defaultServiceAccount, objectLevelWorkloadIdentity, providerName, providerNamespace,
		tokenClient, tokenCache)
	if err != nil {
		return "", err
	}
	opts = append(opts, auth.WithScopes(scope))

	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return "", fmt.Errorf("error parsing proxy URL: %w", err)
		}
		opts = append(opts, auth.WithProxyURL(*proxyURL))
	}

	token, err := auth.GetAccessToken(ctx, azure.Provider{}, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	return token.(*azure.Token).Token, nil
}
