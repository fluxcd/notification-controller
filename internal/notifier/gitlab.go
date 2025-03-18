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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
)

type GitLab struct {
	Id           string
	CommitStatus string
	Client       *gitlab.Client
}

func NewGitLab(commitStatus string, addr string, token string, certPool *x509.CertPool) (*GitLab, error) {
	if len(token) == 0 {
		return nil, errors.New("gitlab token cannot be empty")
	}

	host, id, err := parseGitAddress(addr)
	if err != nil {
		return nil, err
	}

	// this should never happen
	if commitStatus == "" {
		return nil, errors.New("commit status cannot be empty")
	}

	opts := []gitlab.ClientOptionFunc{gitlab.WithBaseURL(host)}
	if certPool != nil {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		}
		hc := &http.Client{Transport: tr}
		opts = append(opts, gitlab.WithHTTPClient(hc))
	}
	client, err := gitlab.NewClient(token, opts...)
	if err != nil {
		return nil, err
	}

	gitlab := &GitLab{
		Id:           id,
		CommitStatus: commitStatus,
		Client:       client,
	}

	return gitlab, nil
}

// Post GitLab commit status
func (g *GitLab) Post(ctx context.Context, event eventv1.Event) error {
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
	state, err := toGitLabState(event.Severity)
	if err != nil {
		return err
	}

	_, desc := formatNameAndDescription(event)
	id := g.CommitStatus
	status := &gitlab.CommitStatus{
		Name:        id,
		SHA:         rev,
		Status:      string(state),
		Description: desc,
	}

	getOpt := &gitlab.GetCommitStatusesOptions{
		Name: &status.Name,
	}
	statuses, _, err := g.Client.Commits.GetCommitStatuses(g.Id, rev, getOpt, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("unable to list commit status: %s", err)
	}
	if duplicateGitlabStatus(statuses, status) {
		return nil
	}

	setOpt := &gitlab.SetCommitStatusOptions{
		Name:        &id,
		Description: &desc,
		State:       state,
	}
	_, _, err = g.Client.Commits.SetCommitStatus(g.Id, rev, setOpt, gitlab.WithContext(ctx))
	if err != nil {
		return err
	}

	return nil
}

func toGitLabState(severity string) (gitlab.BuildStateValue, error) {
	switch severity {
	case eventv1.EventSeverityInfo:
		return gitlab.Success, nil
	case eventv1.EventSeverityError:
		return gitlab.Failed, nil
	default:
		return "", errors.New("can't convert to gitlab state")
	}
}

func duplicateGitlabStatus(statuses []*gitlab.CommitStatus, status *gitlab.CommitStatus) bool {
	for _, s := range statuses {
		if s.SHA == status.SHA {
			if s.Status == status.Status && s.Description == status.Description && s.Name == status.Name {
				return true
			}
		}
	}

	return false
}
