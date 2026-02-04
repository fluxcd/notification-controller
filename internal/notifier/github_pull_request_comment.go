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
	"slices"
	"strconv"
	"strings"

	"github.com/google/go-github/v64/github"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type GitHubPullRequestComment struct {
	Owner       string
	Repo        string
	UserLogin   string
	AppSlug     string
	ProviderUID string
	Client      *github.Client
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
		Owner:       clientInfo.Owner,
		Repo:        clientInfo.Repo,
		UserLogin:   clientInfo.UserLogin,
		AppSlug:     clientInfo.AppSlug,
		ProviderUID: providerUID,
		Client:      clientInfo.Client,
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

// generateCommentKey generates a unique comment key based on the provider UID,
// involved object kind and name.
func (g *GitHubPullRequestComment) generateCommentKey(event *eventv1.Event) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		g.ProviderUID,
		event.InvolvedObject.Kind,
		event.InvolvedObject.Namespace,
		event.InvolvedObject.Name)
}

// formatCommentKeyMarker formats the comment key marker that is used to identify comments
// created by this notifier for a specific event. It is included in the comment body as an
// HTML comment, so it is not visible in the GitHub UI but can be used to find and delete
// existing comments for the same event. The marker includes the generated comment key.
func (g *GitHubPullRequestComment) formatCommentKeyMarker(event *eventv1.Event) string {
	return fmt.Sprintf("<!-- flux-pr-comment-key:%s -->", g.generateCommentKey(event))
}

// formatCommentBody formats the body of the pull request comment based on the event data.
func (g *GitHubPullRequestComment) formatCommentBody(event *eventv1.Event) string {
	marker := g.formatCommentKeyMarker(event)

	// Format severity with emoji
	var severityText string
	if event.Severity == eventv1.EventSeverityError {
		severityText = "⚠️ Error"
	} else {
		severityText = "ℹ️ Info"
	}

	// Format object identifier
	objectID := fmt.Sprintf("%s/%s/%s",
		event.InvolvedObject.Kind,
		event.InvolvedObject.Namespace,
		event.InvolvedObject.Name)

	// Build metadata section
	keys := make([]string, 0, len(event.Metadata))
	for k := range event.Metadata {
		if k != eventv1.MetaChangeRequestKey {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	var metadataLines strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&metadataLines, "* `%s`: `%s`\n", key, event.Metadata[key])
	}

	// Format the comment body
	return fmt.Sprintf("%s\n\n## Flux Status\n\n%s: `%s`\n\n`%s`\n\n%s",
		marker, severityText, objectID, event.Message, metadataLines.String())
}
