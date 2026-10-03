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
	"strconv"
	"strings"

	"code.gitea.io/sdk/gitea"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

type GiteaPullRequestComment struct {
	changeRequestComment
	Owner    string
	Repo     string
	Username string
	Client   *gitea.Client
}

func NewGiteaPullRequestComment(providerUID string, opts ...GiteaClientOption) (*GiteaPullRequestComment, error) {
	if providerUID == "" {
		return nil, errors.New("provider UID cannot be empty")
	}

	opts = append(opts, WithGiteaFetchUserLogin())
	clientInfo, err := NewGiteaClient(opts...)
	if err != nil {
		return nil, err
	}

	return &GiteaPullRequestComment{
		changeRequestComment: changeRequestComment{
			ProviderUID:      providerUID,
			CommentKeyPrefix: "flux-pr-comment-key",
		},
		Owner:    clientInfo.Owner,
		Repo:     clientInfo.Repo,
		Username: clientInfo.Username,
		Client:   clientInfo.Client,
	}, nil
}

// Post posts a comment on the pull request specified in the event metadata.
// If the comment already exists (based on the comment key), it updates the existing comment
// instead of creating a new one.
func (g *GiteaPullRequestComment) Post(ctx context.Context, event eventv1.Event) error {
	body := g.formatCommentBody(&event)
	prNumber, err := g.getPullRequestNumber(&event)
	if err != nil {
		return err
	}

	// List the first 100 comments in the pull request.
	// We fetch only a single page with 100 comments for performance reasons.
	// Since the tendency is for the comment to be created as soon as the PR
	// is opened, it is likely to be in this first page.
	opts := gitea.ListIssueCommentOptions{
		ListOptions: gitea.ListOptions{
			Page:     1,
			PageSize: 100,
		},
	}
	comments, _, err := g.Client.ListIssueComments(g.Owner, g.Repo, int64(prNumber), opts)
	if err != nil {
		return fmt.Errorf("failed to list pull request comments: %w", err)
	}

	// Find comment from the authenticated user matching the comment key
	var existingComment *gitea.Comment
	commentKeyMarker := g.formatCommentKeyMarker(&event)
	for _, c := range comments {
		if c.Poster != nil && c.Poster.UserName == g.Username && strings.Contains(c.Body, commentKeyMarker) {
			existingComment = c
			break
		}
	}

	if existingComment != nil {
		// Existing comment found, update it
		_, _, err = g.Client.EditIssueComment(g.Owner, g.Repo, existingComment.ID, gitea.EditIssueCommentOption{
			Body: body,
		})
		if err != nil {
			return fmt.Errorf("failed to update pull request comment: %w", err)
		}
	} else {
		// No existing comment found, create a new one
		_, _, err = g.Client.CreateIssueComment(g.Owner, g.Repo, int64(prNumber), gitea.CreateIssueCommentOption{
			Body: body,
		})
		if err != nil {
			return fmt.Errorf("failed to create pull request comment: %w", err)
		}
	}

	return nil
}

// getPullRequestNumber extracts the pull request number from the event
// metadata and converts it to an integer. It looks at the key specified
// by eventv1.MetaChangeRequestKey.
func (g *GiteaPullRequestComment) getPullRequestNumber(event *eventv1.Event) (int, error) {
	prStr, ok := event.Metadata[eventv1.MetaChangeRequestKey]
	if !ok {
		return 0, fmt.Errorf("missing %q metadata key", eventv1.MetaChangeRequestKey)
	}
	prNumber, err := strconv.Atoi(prStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %q metadata value %q: %w", eventv1.MetaChangeRequestKey, prStr, err)
	}
	return prNumber, nil
}
