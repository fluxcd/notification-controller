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
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type GitLabMergeRequestComment struct {
	changeRequestComment
	ProjectID string
	Username  string
	Client    *gitlab.Client
}

func NewGitLabMergeRequestComment(providerUID string, addr string, token string, tlsConfig *tls.Config) (*GitLabMergeRequestComment, error) {
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
	if tlsConfig != nil {
		tr := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		hc := &http.Client{Transport: tr}
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
		ProjectID: id,
		Username:  user.Username,
		Client:    client,
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

	// List the first 100 notes in the merge request.
	// We fetch only a single page with 100 notes for performance reasons.
	// Since the tendency is for the comment to be created as soon as the MR
	// is opened, it is likely to be in this first page. 100 is the maximum value for per_page:
	// https://docs.gitlab.com/api/rest/#pagination
	opts := &gitlab.ListMergeRequestNotesOptions{
		Sort:        gitlab.Ptr("asc"),
		OrderBy:     gitlab.Ptr("created_at"),
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	notes, _, err := g.Client.Notes.ListMergeRequestNotes(g.ProjectID, mrIID, opts, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to list merge request notes: %w", err)
	}

	// Find note from the authenticated user matching the comment key
	var note *gitlab.Note
	commentKeyMarker := g.formatCommentKeyMarker(&event)
	for _, n := range notes {
		if n.Author.Username == g.Username && strings.Contains(n.Body, commentKeyMarker) {
			note = n
			break
		}
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
