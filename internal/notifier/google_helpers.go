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

	"google.golang.org/api/option"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/gcp"
	"github.com/fluxcd/pkg/cache"

	"github.com/fluxcd/notification-controller/api/v1beta3"
)

// buildGCPClientOptions builds client options for GCP services.
// Authentication precedence: JSON credentials take priority over workload identity.
func buildGCPClientOptions(ctx context.Context, opts notifierOptions) ([]option.ClientOption, error) {
	var clientOpts []option.ClientOption

	if opts.Token != "" {
		clientOpts = append(clientOpts, option.WithCredentialsJSON([]byte(opts.Token)))
	} else {
		authOpts := []auth.Option{
			auth.WithClient(opts.TokenClient),
			auth.WithServiceAccountNamespace(opts.ProviderNamespace),
		}

		if opts.TokenCache != nil {
			involvedObject := cache.InvolvedObject{
				Kind:      v1beta3.ProviderKind,
				Name:      opts.ProviderName,
				Namespace: opts.ProviderNamespace,
				Operation: OperationPost,
			}
			authOpts = append(authOpts, auth.WithCache(*opts.TokenCache, involvedObject))
		}

		if opts.ServiceAccountName != "" {
			authOpts = append(authOpts, auth.WithServiceAccountName(opts.ServiceAccountName))
		}

		if opts.ProxyURL != "" {
			proxyURL, err := url.Parse(opts.ProxyURL)
			if err != nil {
				return nil, fmt.Errorf("error parsing proxy URL: %w", err)
			}
			authOpts = append(authOpts, auth.WithProxyURL(*proxyURL))
		}

		tokenSource := gcp.NewTokenSource(ctx, authOpts...)
		clientOpts = append(clientOpts, option.WithTokenSource(tokenSource))
	}

	return clientOpts, nil
}
