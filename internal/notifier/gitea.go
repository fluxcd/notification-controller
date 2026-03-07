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
	"errors"
	"fmt"
	"os"

	"code.gitea.io/sdk/gitea"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/fluxcd/pkg/apis/meta"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Gitea struct {
	Owner        string
	Repo         string
	CommitStatus string
	Client       *gitea.Client
	Debug        bool
}

var _ Interface = &Gitea{}

func NewGitea(commitStatus string, opts ...GiteaClientOption) (*Gitea, error) {
	if commitStatus == "" {
		return nil, errors.New("commit status cannot be empty")
	}

	clientInfo, err := NewGiteaClient(opts...)
	if err != nil {
		return nil, err
	}

	return &Gitea{
		Owner:        clientInfo.Owner,
		Repo:         clientInfo.Repo,
		CommitStatus: commitStatus,
		Client:       clientInfo.Client,
		Debug:        os.Getenv("NOTIFIER_GITEA_DEBUG") == "true",
	}, nil
}

func (g *Gitea) Post(ctx context.Context, event eventv1.Event) error {
	revString, ok := event.GetRevision()
	if !ok {
		return errors.New("missing revision metadata")
	}
	rev, err := parseRevision(revString)
	if err != nil {
		return err
	}
	state, err := toGiteaState(event)
	if err != nil {
		return err
	}

	_, desc := formatNameAndDescription(event)
	id := g.CommitStatus

	status := gitea.CreateStatusOption{
		State:       state,
		TargetURL:   "",
		Description: desc,
		Context:     id,
	}

	listStatusesOpts := gitea.ListStatusesOption{
		ListOptions: gitea.ListOptions{
			Page:     0,
			PageSize: 50,
		},
	}
	statuses, _, err := g.Client.ListStatuses(g.Owner, g.Repo, rev, listStatusesOpts)
	if err != nil {
		return fmt.Errorf("could not list commit statuses: %w", err)
	}
	if duplicateGiteaStatus(statuses, &status) {
		if g.Debug {
			ctrl.Log.Info("gitea skip posting duplicate status",
				"owner", g.Owner, "repo", g.Repo, "commit_hash", rev, "status", status)
		}
		return nil
	}

	if g.Debug {
		ctrl.Log.Info("gitea create commit begin",
			"owner", g.Owner, "repo", g.Repo, "event", event, "status", status)
	}

	st, rsp, err := g.Client.CreateStatus(g.Owner, g.Repo, rev, status)
	if err != nil {
		if g.Debug {
			ctrl.Log.Error(err, "gitea create commit failed", "status", status)
		}
		return err
	}

	if g.Debug {
		ctrl.Log.Info("gitea create commit ok", "response", rsp, "response_status", st)
	}

	return nil
}

func toGiteaState(event eventv1.Event) (gitea.StatusState, error) {
	// progressing events
	if event.HasReason(meta.ProgressingReason) {
		// pending
		return gitea.StatusPending, nil
	}
	switch event.Severity {
	case eventv1.EventSeverityInfo:
		return gitea.StatusSuccess, nil
	case eventv1.EventSeverityError:
		return gitea.StatusFailure, nil
	default:
		return gitea.StatusError, errors.New("can't convert to gitea state")
	}
}

// duplicateGiteaStatus return true if the latest status
// with a matching context has the same state and description
func duplicateGiteaStatus(statuses []*gitea.Status, status *gitea.CreateStatusOption) bool {
	if status == nil || statuses == nil {
		return false
	}

	for _, s := range statuses {
		if s.Context == "" || s.State == "" || s.Description == "" {
			continue
		}

		if s.Context == status.Context {
			if s.State == status.State && s.Description == status.Description {
				return true
			}

			return false
		}
	}

	return false
}
