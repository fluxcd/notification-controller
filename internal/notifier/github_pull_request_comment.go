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

	"github.com/google/go-github/v64/github"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type GitHubPullRequestComment struct {
	changeRequestComment
	Owner     string
	Repo      string
	UserLogin string
	AppSlug   string
	Client    *github.Client
}

func NewGitHubPullRequestComment(ctx context.Context, providerUID string, opts ...GitHubClientOption) (*GitHubPullRequestComment, error) {
	if providerUID == "" {
		return nil, errors.New("provider UID cannot be empty")
	}

	opts = append(opts, WithGitHubFetchUserLogin())
	clientInfo, err := NewGitHubClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &GitHubPullRequestComment{
		changeRequestComment: changeRequestComment{
			ProviderUID:      providerUID,
			CommentKeyPrefix: "flux-pr-comment-key",
		},
		Owner:     clientInfo.Owner,
		Repo:      clientInfo.Repo,
		UserLogin: clientInfo.UserLogin,
		AppSlug:   clientInfo.AppSlug,
		Client:    clientInfo.Client,
	}, nil
}

// Post posts a comment on the pull request specified in the event metadata.
// If the comment already exists (based on the comment key), it updates the existing comment
// instead of creating a new one.
func (g *GitHubPullRequestComment) Post(ctx context.Context, event eventv1.Event) error {
	body := g.formatCommentBody(&event)
	prNumber, err := g.getPullRequestNumber(&event)
	if err != nil {
		return err
	}

	// List the first 100 comments in the pull request.
	// We fetch only a single page with 100 comments for performance reasons.
	// Since the tendency is for the comment to be created as soon as the PR
	// is opened, it is likely to be in this first page. 100 is the maximum value for per_page:
	// https://docs.github.com/en/rest/issues/comments?apiVersion=2022-11-28#list-issue-comments
	opts := &github.IssueListCommentsOptions{
		Sort:        github.String("created"),
		Direction:   github.String("asc"),
		ListOptions: github.ListOptions{PerPage: 100},
	}
	comments, _, err := g.Client.Issues.ListComments(ctx, g.Owner, g.Repo, prNumber, opts)
	if err != nil {
		return fmt.Errorf("failed to list pull request comments: %w", err)
	}

	// Find comment from the authenticated user matching the comment key
	var comment *github.IssueComment
	userLogin := g.getUserLogin()
	commentKeyMarker := g.formatCommentKeyMarker(&event)
	for _, c := range comments {
		if c.GetUser().GetLogin() == userLogin && strings.Contains(c.GetBody(), commentKeyMarker) {
			comment = c
			break
		}
	}

	if comment != nil {
		// Existing comment found, update it
		_, _, err = g.Client.Issues.EditComment(ctx, g.Owner, g.Repo, comment.GetID(), &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			return fmt.Errorf("failed to update pull request comment: %w", err)
		}
	} else {
		// No existing comment found, create a new one
		_, _, err = g.Client.Issues.CreateComment(ctx, g.Owner, g.Repo, prNumber, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			return fmt.Errorf("failed to create pull request comment: %w", err)
		}
	}

	return nil
}

// getUserLogin returns the login of the authenticated user (PAT or GitHub App)
// as it appears in the GitHub API object representing pull request comments.
func (g *GitHubPullRequestComment) getUserLogin() string {
	if g.AppSlug != "" {
		// GitHub App comments appear as "<app-slug>[bot]".
		return fmt.Sprintf("%s[bot]", g.AppSlug)
	}
	return g.UserLogin
}

// getPullRequestNumber extracts the pull request number from the event
// metadata and converts it to an integer. It looks at the key specified
// by eventv1.MetaChangeRequestKey.
func (g *GitHubPullRequestComment) getPullRequestNumber(event *eventv1.Event) (int, error) {
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
