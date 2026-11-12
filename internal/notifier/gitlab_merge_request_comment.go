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
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

type GitLabMergeRequestComment struct {
	changeRequestComment
	ProjectID  string
	Username   string
	Client     *gitlab.Client
	FetchPages bool
}

func NewGitLabMergeRequestComment(providerUID string, addr string, proxyURL *url.URL, token string, tlsConfig *tls.Config, fetchPages bool) (*GitLabMergeRequestComment, error) {
	if providerUID == "" {
		return nil, errors.New("provider UID cannot be empty")
	}

	if token == "" {
		return nil, errors.New("gitlab token cannot be empty")
	}

	host, id, err := parseGitAddress(addr)
	if err != nil {
		return nil, err
	}

	opts := []gitlab.ClientOptionFunc{gitlab.WithBaseURL(host)}
	if proxyURL != nil || tlsConfig != nil {
		hc := &http.Client{Transport: newTransport(proxyURL, tlsConfig)}
		opts = append(opts, gitlab.WithHTTPClient(hc))
	}

	client, err := gitlab.NewClient(token, opts...)
	if err != nil {
		return nil, err
	}

	// Fetch the authenticated user's username
	user, _, err := client.Users.CurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated user info: %w", err)
	}

	return &GitLabMergeRequestComment{
		changeRequestComment: changeRequestComment{
			ProviderUID:      providerUID,
			CommentKeyPrefix: "flux-mr-comment-key",
		},
		ProjectID:  id,
		Username:   user.Username,
		Client:     client,
		FetchPages: fetchPages,
	}, nil
}

// Post posts a comment on the merge request specified in the event metadata.
// If the comment already exists (based on the comment key), it updates the existing comment
// instead of creating a new one.
func (g *GitLabMergeRequestComment) Post(ctx context.Context, event eventv1.Event) error {
	body := g.formatCommentBody(&event)
	mrIID, err := g.getMergeRequestIID(&event)
	if err != nil {
		return err
	}

	// Fetch only the first page by default to limit API requests.
	opts := &gitlab.ListMergeRequestNotesOptions{
		Sort:        gitlab.Ptr("asc"),
		OrderBy:     gitlab.Ptr("created_at"),
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	var note *gitlab.Note
	commentKeyMarker := g.formatCommentKeyMarker(&event)
	requestOpts := []gitlab.RequestOptionFunc{gitlab.WithContext(ctx)}
	for {
		notes, response, err := g.Client.Notes.ListMergeRequestNotes(g.ProjectID, mrIID, opts, requestOpts...)
		if err != nil {
			return fmt.Errorf("failed to list merge request notes: %w", err)
		}
		for _, n := range notes {
			if n.Author.Username == g.Username && strings.Contains(n.Body, commentKeyMarker) {
				note = n
				break
			}
		}
		if note != nil || !g.FetchPages {
			break
		}
		next, ok := gitlab.WithNext(response)
		if !ok {
			break
		}
		requestOpts = []gitlab.RequestOptionFunc{gitlab.WithContext(ctx), next}
	}

	if note != nil {
		// Existing note found, update it
		_, _, err = g.Client.Notes.UpdateMergeRequestNote(g.ProjectID, mrIID, note.ID, &gitlab.UpdateMergeRequestNoteOptions{
			Body: &body,
		}, gitlab.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("failed to update merge request note: %w", err)
		}
	} else {
		// No existing note found, create a new one
		_, _, err = g.Client.Notes.CreateMergeRequestNote(g.ProjectID, mrIID, &gitlab.CreateMergeRequestNoteOptions{
			Body: &body,
		}, gitlab.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("failed to create merge request note: %w", err)
		}
	}

	return nil
}

// getMergeRequestIID extracts the merge request IID from the event
// metadata and converts it to an int64. It looks at the key specified
// by eventv1.MetaChangeRequestKey.
func (g *GitLabMergeRequestComment) getMergeRequestIID(event *eventv1.Event) (int64, error) {
	mrStr, ok := event.Metadata[eventv1.MetaChangeRequestKey]
	if !ok {
		return 0, fmt.Errorf("missing %q metadata key", eventv1.MetaChangeRequestKey)
	}
	mrIID, err := strconv.ParseInt(mrStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %q metadata value %q: %w", eventv1.MetaChangeRequestKey, mrStr, err)
	}
	return mrIID, nil
}
