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
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v64/github"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type GitHubDispatch struct {
	Owner  string
	Repo   string
	Client *github.Client
}

func NewGitHubDispatch(ctx context.Context, opts ...GitHubClientOption) (*GitHubDispatch, error) {
	clientInfo, err := NewGitHubClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &GitHubDispatch{
		Owner:  clientInfo.Owner,
		Repo:   clientInfo.Repo,
		Client: clientInfo.Client,
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

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal object into json: %w", err)
	}
	clientPayload := json.RawMessage(b)

	opts := github.DispatchRequestOptions{
		EventType:     eventType,
		ClientPayload: &clientPayload,
	}
	_, _, err = g.Client.Repositories.Dispatch(ctx, g.Owner, g.Repo, opts)
	if err != nil {
		return fmt.Errorf("could not send github repository dispatch webhook: %v", err)
	}

	return nil
}
