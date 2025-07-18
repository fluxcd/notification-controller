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
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	gogithub "github.com/google/go-github/v64/github"
	"golang.org/x/oauth2"

	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/git/github"

	"github.com/fluxcd/notification-controller/api/v1beta3"
)

// repoInfo is an internal type encapsulating owner, repo and client
type repoInfo struct {
	owner  string
	repo   string
	client *gogithub.Client
}

// getGitHubAppOptions constructs the github app authentication options.
func getGitHubAppOptions(providerName, providerNamespace, proxy string,
	secretData map[string][]byte, tokenCache *cache.TokenCache) ([]github.OptFunc, error) {

	githubOpts := []github.OptFunc{
		github.WithAppData(secretData),
	}

	if len(githubOpts) > 0 && proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("error parsing proxy URL '%s': %w", proxy, err)
		}
		githubOpts = append(githubOpts, github.WithProxyURL(proxyURL))
	}

	if len(githubOpts) > 0 && tokenCache != nil {
		githubOpts = append(githubOpts, github.WithCache(tokenCache,
			v1beta3.ProviderKind, providerName, providerNamespace, OperationPost))
	}

	return githubOpts, nil
}

// getRepoInfoAndGithubClient gets the github client and repository info used by Github and GithubDispatch providers
func getRepoInfoAndGithubClient(addr string, token string, tlsConfig *tls.Config,
	proxyURL string, providerName string, providerNamespace string,
	secretData map[string][]byte, tokenCache *cache.TokenCache) (*repoInfo, error) {

	if len(token) == 0 {
		if _, ok := secretData[github.KeyAppID]; !ok {
			return nil, errors.New("github token or github app details must be specified")
		}

		githubOpts, err := getGitHubAppOptions(providerName, providerNamespace, proxyURL, secretData, tokenCache)
		if err != nil {
			return nil, err
		}

		client, err := github.New(githubOpts...)
		if err != nil {
			return nil, err
		}

		appToken, err := client.GetToken(context.Background())
		if err != nil {
			return nil, err
		}
		token = appToken.Token
	}

	host, id, err := parseGitAddress(addr)
	if err != nil {
		return nil, err
	}

	comp := strings.Split(id, "/")
	if len(comp) != 2 {
		return nil, fmt.Errorf("invalid repository id %q", id)
	}

	baseUrl, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	client := gogithub.NewClient(tc)
	if baseUrl.Host != "github.com" {
		if tlsConfig != nil {
			tr := &http.Transport{
				TLSClientConfig: tlsConfig,
			}
			hc := &http.Client{Transport: tr}
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, hc)
			tc = oauth2.NewClient(ctx, ts)
		}
		client, err = gogithub.NewClient(tc).WithEnterpriseURLs(host, host)
		if err != nil {
			return nil, fmt.Errorf("could not create enterprise GitHub client: %v", err)
		}
	}

	return &repoInfo{comp[0], comp[1], client}, nil
}
