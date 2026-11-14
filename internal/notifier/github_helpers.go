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

	gogithub "github.com/google/go-github/v90/github"
	"golang.org/x/oauth2"

	"github.com/fluxcd/pkg/auth/githubapp"
	"github.com/fluxcd/pkg/cache"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

// GitHubClient holds the GitHub client and repository information.
type GitHubClient struct {
	UserLogin string
	AppSlug   string
	Owner     string
	Repo      string
	Client    *gogithub.Client
}

// NewGitHubClient creates a new GitHubClient with the provided options.
func NewGitHubClient(ctx context.Context, opts *notifierOptions, fetchUserLogin bool) (*GitHubClient, error) {
	// Get GitHub App token if a token is not provided, and app details are available.
	var appSlug string
	token := opts.Token
	if token == "" {
		token = opts.Password
	}
	if token == "" {
		if _, ok := opts.SecretData[githubapp.KeyAppID]; !ok {
			return nil, errors.New("github token or github app details must be specified")
		}

		appToken, err := getGitHubAppToken(ctx, opts, fetchUserLogin)
		if err != nil {
			return nil, err
		}
		token = appToken.Token
		appSlug = appToken.Slug
	}

	// Parse the GitHub address to extract host, owner, and repo information.
	host, id, err := parseGitAddress(opts.URL)
	if err != nil {
		return nil, err
	}
	ownerAndRepo := strings.Split(id, "/")
	if len(ownerAndRepo) != 2 {
		return nil, fmt.Errorf("invalid repository id: '%s'", id)
	}

	// Create client.
	client, err := createGitHubClient(ctx, host, token, opts.ProxyURL, opts.TLSConfig)
	if err != nil {
		return nil, err
	}

	// Fetch user login if needed.
	var userLogin string
	if fetchUserLogin && appSlug == "" {
		myUser, _, err := client.Users.Get(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get authenticated user info: %w", err)
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
func getGitHubAppToken(ctx context.Context, opts *notifierOptions, fetchUserLogin bool) (*githubapp.AppToken, error) {
	githubOpts := []githubapp.OptFunc{
		githubapp.WithAppData(opts.SecretData),
	}

	if opts.ProxyURL != nil {
		githubOpts = append(githubOpts, githubapp.WithProxyURL(opts.ProxyURL))
	}

	if opts.TokenCache != nil {
		githubOpts = append(githubOpts, githubapp.WithCache(opts.TokenCache, cache.InvolvedObject{
			Kind:      apiv1.ProviderKind,
			Name:      opts.ProviderName,
			Namespace: opts.ProviderNamespace,
			Operation: OperationPost,
		}))
	}

	if fetchUserLogin {
		githubOpts = append(githubOpts, githubapp.WithAppSlugReflection())
	}

	if opts.TLSConfig != nil {
		githubOpts = append(githubOpts, githubapp.WithTLSConfig(opts.TLSConfig))
	}

	client, err := githubapp.New(githubOpts...)
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
func createGitHubClient(ctx context.Context, host, token string, proxyURL *url.URL, tlsConfig *tls.Config) (*gogithub.Client, error) {
	baseURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	hc := &http.Client{Transport: newTransport(proxyURL, tlsConfig)}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, hc)
	tc := oauth2.NewClient(ctx, ts)

	if baseURL.Host == "github.com" {
		return gogithub.NewClient(gogithub.WithHTTPClient(tc))
	}

	// Enterprise GitHub
	client, err := gogithub.NewClient(gogithub.WithHTTPClient(tc), gogithub.WithEnterpriseURLs(host, host))
	if err != nil {
		return nil, fmt.Errorf("failed to create enterprise GitHub client: %w", err)
	}

	return client, nil
}
