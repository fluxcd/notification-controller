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

// GitHubClient holds the GitHub client and repository information.
type GitHubClient struct {
	UserLogin string
	AppSlug   string
	Owner     string
	Repo      string
	Client    *gogithub.Client
}

// gitHubClientOptions holds the configuration for creating a GitHub client.
type gitHubClientOptions struct {
	address           string
	token             string
	tlsConfig         *tls.Config
	proxyURL          string
	providerName      string
	providerNamespace string
	fetchUserLogin    bool
	secretData        map[string][]byte
	tokenCache        *cache.TokenCache
}

// GitHubClientOption is a functional option for configuring GitHub client creation.
type GitHubClientOption func(*gitHubClientOptions)

// WithGitHubAddress sets the GitHub repository address.
func WithGitHubAddress(addr string) GitHubClientOption {
	return func(o *gitHubClientOptions) {
		o.address = addr
	}
}

// WithGitHubToken sets the authentication token.
func WithGitHubToken(token string) GitHubClientOption {
	return func(o *gitHubClientOptions) {
		o.token = token
	}
}

// WithGitHubTLSConfig sets the TLS configuration.
func WithGitHubTLSConfig(cfg *tls.Config) GitHubClientOption {
	return func(o *gitHubClientOptions) {
		o.tlsConfig = cfg
	}
}

// WithGitHubProxyURL sets the proxy URL.
func WithGitHubProxyURL(proxyURL string) GitHubClientOption {
	return func(o *gitHubClientOptions) {
		o.proxyURL = proxyURL
	}
}

// WithGitHubProvider sets the provider name and namespace for token caching.
func WithGitHubProvider(name, namespace string) GitHubClientOption {
	return func(o *gitHubClientOptions) {
		o.providerName = name
		o.providerNamespace = namespace
	}
}

// WithGitHubFetchUserLogin enables fetching the authenticated user's login.
// This is needed for providers that need to identify their own comments/actions.
func WithGitHubFetchUserLogin() GitHubClientOption {
	return func(o *gitHubClientOptions) {
		o.fetchUserLogin = true
	}
}

// WithGitHubSecretData sets the secret data for GitHub App authentication.
func WithGitHubSecretData(data map[string][]byte) GitHubClientOption {
	return func(o *gitHubClientOptions) {
		o.secretData = data
	}
}

// WithGitHubTokenCache sets the token cache for GitHub App authentication.
func WithGitHubTokenCache(tokenCache *cache.TokenCache) GitHubClientOption {
	return func(o *gitHubClientOptions) {
		o.tokenCache = tokenCache
	}
}

// NewGitHubClient creates a new GitHubClientInfo with the provided options.
func NewGitHubClient(ctx context.Context, opts ...GitHubClientOption) (*GitHubClient, error) {
	var o gitHubClientOptions
	for _, opt := range opts {
		opt(&o)
	}

	// Get GitHub App token if a token is not provided, and app details are available.
	var appSlug string
	token := o.token
	if token == "" {
		if _, ok := o.secretData[github.KeyAppID]; !ok {
			return nil, errors.New("github token or github app details must be specified")
		}

		appToken, err := getGitHubAppToken(ctx, &o)
		if err != nil {
			return nil, err
		}
		token = appToken.Token
		appSlug = appToken.Slug
	}

	// Parse the GitHub address to extract host, owner, and repo information.
	host, id, err := parseGitAddress(o.address)
	if err != nil {
		return nil, err
	}
	ownerAndRepo := strings.Split(id, "/")
	if len(ownerAndRepo) != 2 {
		return nil, fmt.Errorf("invalid repository id: '%s'", id)
	}

	// Create client.
	client, err := createGitHubClient(ctx, host, token, o.tlsConfig)
	if err != nil {
		return nil, err
	}

	// Fetch user login if needed.
	var userLogin string
	if o.fetchUserLogin && appSlug == "" {
		myUser, _, err := client.Users.Get(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get authenticated user info: %v", err)
		}
		userLogin = myUser.GetLogin()
	}

	return &GitHubClient{
		UserLogin: userLogin,
		AppSlug:   appSlug,
		Owner:     ownerAndRepo[0],
		Repo:      ownerAndRepo[1],
		Client:    client,
	}, nil
}

// getGitHubAppToken retrieves an installation token using GitHub App authentication.
func getGitHubAppToken(ctx context.Context, o *gitHubClientOptions) (*github.AppToken, error) {
	githubOpts := []github.OptFunc{
		github.WithAppData(o.secretData),
	}

	if o.proxyURL != "" {
		proxyURL, err := url.Parse(o.proxyURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing proxy URL '%s': %w", o.proxyURL, err)
		}
		githubOpts = append(githubOpts, github.WithProxyURL(proxyURL))
	}

	if o.tokenCache != nil {
		githubOpts = append(githubOpts, github.WithCache(o.tokenCache,
			v1beta3.ProviderKind, o.providerName, o.providerNamespace, OperationPost))
	}

	if o.fetchUserLogin {
		githubOpts = append(githubOpts, github.WithAppSlugReflection())
	}

	if o.tlsConfig != nil {
		githubOpts = append(githubOpts, github.WithTLSConfig(o.tlsConfig))
	}

	client, err := github.New(githubOpts...)
	if err != nil {
		return nil, err
	}

	appToken, err := client.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	return appToken, nil
}

// createGitHubClient creates a GitHub API client for the given host.
func createGitHubClient(ctx context.Context, host, token string, tlsConfig *tls.Config) (*gogithub.Client, error) {
	baseURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	if baseURL.Host == "github.com" {
		return gogithub.NewClient(tc), nil
	}

	// Enterprise GitHub
	if tlsConfig != nil {
		tr := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		hc := &http.Client{Transport: tr}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, hc)
		tc = oauth2.NewClient(ctx, ts)
	}

	client, err := gogithub.NewClient(tc).WithEnterpriseURLs(host, host)
	if err != nil {
		return nil, fmt.Errorf("failed to create enterprise GitHub client: %v", err)
	}

	return client, nil
}

// GitHubClientOptions returns the GitHub client options derived from notifierOptions.
// This handles the token/password fallback logic and converts factory options to GitHub client options.
func (o *notifierOptions) GitHubClientOptions() []GitHubClientOption {
	token := o.Token
	if token == "" && o.Password != "" {
		token = o.Password
	}
	return []GitHubClientOption{
		WithGitHubAddress(o.URL),
		WithGitHubToken(token),
		WithGitHubTLSConfig(o.TLSConfig),
		WithGitHubProxyURL(o.ProxyURL),
		WithGitHubProvider(o.ProviderName, o.ProviderNamespace),
		WithGitHubSecretData(o.SecretData),
		WithGitHubTokenCache(o.TokenCache),
	}
}
