/*
Copyright 2020 The Flux authors

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

	"github.com/google/go-github/v64/github"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
)

type GitHub struct {
	Owner        string
	Repo         string
	CommitStatus string
	Client       *github.Client
}

func NewGitHub(ctx context.Context, commitStatus string, opts ...GitHubClientOption) (*GitHub, error) {
	if commitStatus == "" {
		return nil, errors.New("commit status cannot be empty")
	}

	clientInfo, err := NewGitHubClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &GitHub{
		Owner:        clientInfo.Owner,
		Repo:         clientInfo.Repo,
		CommitStatus: commitStatus,
		Client:       clientInfo.Client,
	}, nil
}

// Post Github commit status
func (g *GitHub) Post(ctx context.Context, event eventv1.Event) error {
	// Skip progressing events
	if event.HasReason(meta.ProgressingReason) {
		return nil
	}

	revString, ok := event.GetRevision()
	if !ok {
		return errors.New("missing revision metadata")
	}
	rev, err := parseRevision(revString)
	if err != nil {
		return err
	}
	state, err := toGitHubState(event.Severity)
	if err != nil {
		return err
	}

	_, desc := formatNameAndDescription(event)
	id := g.CommitStatus
	status := &github.RepoStatus{
		State:       &state,
		Context:     &id,
		Description: &desc,
	}

	dup, err := g.isDuplicateStatus(ctx, rev, status)
	if err != nil {
		return err
	}
	if dup {
		return nil
	}

	_, _, err = g.Client.Repositories.CreateStatus(ctx, g.Owner, g.Repo, rev, status)
	if err != nil {
		return fmt.Errorf("could not create commit status: %v", err)
	}

	return nil
}

// isDuplicateStatus reports whether the latest status on rev with the
// context of status already has its state and description.
//
// It reads the combined status, which holds the latest status of each
// context, rather than the list of all statuses: every event for a context
// adds to that list, so once other contexts have posted enough statuses on
// the same commit, the previous status of this context is no longer on the
// first page and would not be found.
func (g *GitHub) isDuplicateStatus(ctx context.Context, rev string, status *github.RepoStatus) (bool, error) {
	opts := &github.ListOptions{PerPage: 100}
	for {
		combined, resp, err := g.Client.Repositories.GetCombinedStatus(ctx, g.Owner, g.Repo, rev, opts)
		if err != nil {
			return false, fmt.Errorf("could not get combined commit status: %w", err)
		}
		for _, s := range combined.Statuses {
			if s.GetContext() == status.GetContext() {
				return duplicateGithubStatus([]*github.RepoStatus{s}, status), nil
			}
		}
		if resp.NextPage == 0 {
			return false, nil
		}
		opts.Page = resp.NextPage
	}
}

func toGitHubState(severity string) (string, error) {
	switch severity {
	case eventv1.EventSeverityInfo:
		return "success", nil
	case eventv1.EventSeverityError:
		return "failure", nil
	default:
		return "", errors.New("can't convert to github state")
	}
}

// duplicateStatus return true if the latest status
// with a matching context has the same state and description
func duplicateGithubStatus(statuses []*github.RepoStatus, status *github.RepoStatus) bool {
	if status == nil || statuses == nil {
		return false
	}

	for _, s := range statuses {
		if s.Context == nil || s.State == nil || s.Description == nil {
			continue
		}

		if *s.Context == *status.Context {
			if *s.State == *status.State && *s.Description == *status.Description {
				return true
			}

			return false
		}
	}

	return false
}
