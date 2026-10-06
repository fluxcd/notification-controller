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
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
)

// GiteaClient holds the Gitea client and repository information.
type GiteaClient struct {
	Owner    string
	Repo     string
	Username string
	Client   *gitea.Client
}

// giteaClientOptions holds the configuration for creating a Gitea client.
type giteaClientOptions struct {
	address        string
	token          string
	proxyURL       string
	tlsConfig      *tls.Config
	fetchUserLogin bool
}

// GiteaClientOption is a functional option for configuring Gitea client creation.
type GiteaClientOption func(*giteaClientOptions)

// WithGiteaAddress sets the Gitea repository address.
func WithGiteaAddress(addr string) GiteaClientOption {
	return func(o *giteaClientOptions) {
		o.address = addr
	}
}

// WithGiteaToken sets the authentication token.
func WithGiteaToken(token string) GiteaClientOption {
	return func(o *giteaClientOptions) {
		o.token = token
	}
}

// WithGiteaProxyURL sets the proxy URL.
func WithGiteaProxyURL(proxyURL string) GiteaClientOption {
	return func(o *giteaClientOptions) {
		o.proxyURL = proxyURL
	}
}

// WithGiteaTLSConfig sets the TLS configuration.
func WithGiteaTLSConfig(cfg *tls.Config) GiteaClientOption {
	return func(o *giteaClientOptions) {
		o.tlsConfig = cfg
	}
}

// WithGiteaFetchUserLogin enables fetching the authenticated user's login.
// This is needed for providers that need to identify their own comments.
func WithGiteaFetchUserLogin() GiteaClientOption {
	return func(o *giteaClientOptions) {
		o.fetchUserLogin = true
	}
}

// giteaIdleConnTimeout overrides the idle timeout of the cloned default
// transport when set. Tests use it to avoid waiting for the default.
var giteaIdleConnTimeout time.Duration

// NewGiteaClient creates a new GiteaClient with the provided options.
func NewGiteaClient(opts ...GiteaClientOption) (*GiteaClient, error) {
	var o giteaClientOptions
	for _, opt := range opts {
		opt(&o)
	}

	if o.token == "" {
		return nil, errors.New("gitea token cannot be empty")
	}

	host, id, err := parseGitAddress(o.address)
	if err != nil {
		return nil, err
	}

	if _, err := url.Parse(host); err != nil {
		return nil, fmt.Errorf("failed parsing host: %w", err)
	}

	idComponents := strings.Split(id, "/")
	if len(idComponents) != 2 {
		return nil, fmt.Errorf("invalid repository id %q", id)
	}

	// A client is created per event, so idle connections must not outlive it.
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if giteaIdleConnTimeout > 0 {
		tr.IdleConnTimeout = giteaIdleConnTimeout
	}
	if o.tlsConfig != nil {
		tr.TLSClientConfig = o.tlsConfig
	}

	if o.proxyURL != "" {
		parsedProxyURL, err := url.Parse(o.proxyURL)
		if err != nil {
			return nil, errors.New("invalid proxy URL")
		}
		tr.Proxy = http.ProxyURL(parsedProxyURL)
	}

	client, err := gitea.NewClient(host, gitea.SetToken(o.token), gitea.SetHTTPClient(&http.Client{Transport: tr}))
	if err != nil {
		return nil, fmt.Errorf("failed creating Gitea client: %w", err)
	}

	var username string
	if o.fetchUserLogin {
		user, _, err := client.GetMyUserInfo()
		if err != nil {
			return nil, fmt.Errorf("failed to get authenticated user info: %w", err)
		}
		username = user.UserName
	}

	return &GiteaClient{
		Owner:    idComponents[0],
		Repo:     idComponents[1],
		Username: username,
		Client:   client,
	}, nil
}

// GiteaClientOptions returns the Gitea client options derived from notifierOptions.
// This handles the token/password fallback logic and converts factory options to Gitea client options.
func (o *notifierOptions) GiteaClientOptions() []GiteaClientOption {
	token := o.Token
	if token == "" && o.Password != "" {
		token = o.Password
	}
	return []GiteaClientOption{
		WithGiteaAddress(o.URL),
		WithGiteaToken(token),
		WithGiteaProxyURL(o.ProxyURL),
		WithGiteaTLSConfig(o.TLSConfig),
	}
}
