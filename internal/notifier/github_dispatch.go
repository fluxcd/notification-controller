/*
Copyright 2022 The Flux authors

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
	"crypto/x509"
	"encoding/json"
	"fmt"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/cache"

	"github.com/google/go-github/v64/github"
)

type GitHubDispatch struct {
	Owner  string
	Repo   string
	Client *github.Client
}

func NewGitHubDispatch(addr string, token string, certPool *x509.CertPool, proxyURL string,
	providerName string, providerNamespace string, secretData map[string][]byte,
	tokenCache *cache.TokenCache) (*GitHubDispatch, error) {

	repoInfo, err := getRepoInfoAndGithubClient(addr, token, certPool,
		proxyURL, providerName, providerNamespace, secretData, tokenCache)
	if err != nil {
		return nil, err
	}

	return &GitHubDispatch{
		Owner:  repoInfo.owner,
		Repo:   repoInfo.repo,
		Client: repoInfo.client,
	}, nil
}

// Post GitHub Repository Dispatch webhook
func (g *GitHubDispatch) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	eventType := fmt.Sprintf("%s/%s.%s",
		event.InvolvedObject.Kind, event.InvolvedObject.Name, event.InvolvedObject.Namespace)

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal object into json: %w", err)
	}
	eventDataRaw := json.RawMessage(eventData)

	opts := github.DispatchRequestOptions{
		EventType:     eventType,
		ClientPayload: &eventDataRaw,
	}
	_, _, err = g.Client.Repositories.Dispatch(ctx, g.Owner, g.Repo, opts)

	if err != nil {
		return fmt.Errorf("could not send github repository dispatch webhook: %v", err)
	}

	return nil
}
