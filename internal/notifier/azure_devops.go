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
	"github.com/fluxcd/pkg/runtime/events"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

const genre string = "fluxcd"

// AzureDevOps is a Azure DevOps notifier.
type AzureDevOps struct {
	Project string
	Repo    string
	Client  git.Client
}

// NewAzureDevOps creates and returns a new AzureDevOps notifier.
func NewAzureDevOps(addr string, token string) (*AzureDevOps, error) {
	if len(token) == 0 {
		return nil, errors.New("azure devops token cannot be empty")
	}

	host, id, err := parseGitAddress(addr)
	if err != nil {
		return nil, err
	}

	comp := strings.Split(id, "/")
	if len(comp) != 4 {
		return nil, fmt.Errorf("invalid repository id %q", id)
	}
	org := comp[0]
	proj := comp[1]
	repo := comp[3]

	orgURL := fmt.Sprintf("%v/%v", host, org)
	connection := azuredevops.NewPatConnection(orgURL, token)
	client := connection.GetClientByUrl(orgURL)
	gitClient := &git.ClientImpl{
		Client: *client,
	}
	return &AzureDevOps{
		Project: proj,
		Repo:    repo,
		Client:  gitClient,
	}, nil
}

// Post Azure DevOps commit status
func (a AzureDevOps) Post(event events.Event) error {
	// Skip progressing events
	if event.Reason == "Progressing" {
		return nil
	}

	revString, ok := event.Metadata["revision"]
	if !ok {
		return errors.New("missing revision metadata")
	}
	rev, err := parseRevision(revString)
	if err != nil {
		return err
	}
	state, err := toAzureDevOpsState(event.Severity)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Check if the exact status is already set
	g := genre
	name, desc := formatNameAndDescription(event)
	createArgs := git.CreateCommitStatusArgs{
		Project:      &a.Project,
		RepositoryId: &a.Repo,
		CommitId:     &rev,
		GitCommitStatusToCreate: &git.GitStatus{
			Description: &desc,
			State:       &state,
			Context: &git.GitStatusContext{
				Genre: &g,
				Name:  &name,
			},
		},
	}
	getArgs := git.GetStatusesArgs{
		Project:      &a.Project,
		RepositoryId: &a.Repo,
		CommitId:     &rev,
	}
	statuses, err := a.Client.GetStatuses(ctx, getArgs)
	if err != nil {
		return fmt.Errorf("could not list commit statuses: %v", err)
	}
	if duplicateAzureDevOpsStatus(statuses, createArgs.GitCommitStatusToCreate) {
		return nil
	}

	// Create a new status
	_, err = a.Client.CreateCommitStatus(context.Background(), createArgs)
	if err != nil {
		return fmt.Errorf("could not create commit status: %v", err)
	}
	return nil
}

func toAzureDevOpsState(severity string) (git.GitStatusState, error) {
	switch severity {
	case events.EventSeverityInfo:
		return git.GitStatusStateValues.Succeeded, nil
	case events.EventSeverityError:
		return git.GitStatusStateValues.Error, nil
	default:
		return "", errors.New("can't convert to azure devops state")
	}
}

// duplicateStatus return true if the latest status
// with a matching context has the same state and description
func duplicateAzureDevOpsStatus(statuses *[]git.GitStatus, status *git.GitStatus) bool {
	for _, s := range *statuses {
		if *s.Context.Name == *status.Context.Name && *s.Context.Genre == *status.Context.Genre {
			if *s.State == *status.State && *s.Description == *status.Description {
				return true
			}

			return false
		}
	}

	return false
}
