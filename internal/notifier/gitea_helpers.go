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

// giteaIdleConnTimeout overrides the idle timeout of the cloned default
// transport when set. Tests use it to avoid waiting for the default.
var giteaIdleConnTimeout time.Duration

// NewGiteaClient creates a new GiteaClient with the provided options.
func NewGiteaClient(ctx context.Context, opts *notifierOptions, fetchUserLogin bool) (*GiteaClient, error) {
	token := opts.Token
	if token == "" {
		token = opts.Password
	}
	if token == "" {
		return nil, errors.New("gitea token cannot be empty")
	}

	host, id, err := parseGitAddress(opts.URL)
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
	tr := newTransport(opts.ProxyURL, opts.TLSConfig)
	if giteaIdleConnTimeout > 0 {
		tr.IdleConnTimeout = giteaIdleConnTimeout
	}

	client, err := gitea.NewClient(host, gitea.SetContext(ctx), gitea.SetToken(token), gitea.SetHTTPClient(&http.Client{Transport: tr}))
	if err != nil {
		return nil, fmt.Errorf("failed creating Gitea client: %w", err)
	}

	var username string
	if fetchUserLogin {
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
