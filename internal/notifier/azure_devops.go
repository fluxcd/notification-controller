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
	"strings"

	"github.com/fluxcd/pkg/recorder"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

const genre string = "fluxcd"

// AzureDevOps is a Azure DevOps notifier.
type AzureDevOps struct {
	Project    string
	Repo       string
	Connection *azuredevops.Connection
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

	c := azuredevops.NewPatConnection(fmt.Sprintf("%v/%v", host, org), token)
	return &AzureDevOps{
		Project:    proj,
		Repo:       repo,
		Connection: c,
	}, nil
}

// Post Azure DevOps commit status
func (a AzureDevOps) Post(event recorder.Event) error {
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

	ctx := context.Background()
	client, err := git.NewClient(ctx, a.Connection)
	if err != nil {
		return err
	}

	g := genre
	name, desc := formatNameAndDescription(event)
	args := git.CreateCommitStatusArgs{
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
	_, err = client.CreateCommitStatus(ctx, args)
	if err != nil {
		return err
	}

	return nil
}

func toAzureDevOpsState(severity string) (git.GitStatusState, error) {
	switch severity {
	case recorder.EventSeverityInfo:
		return git.GitStatusStateValues.Succeeded, nil
	case recorder.EventSeverityError:
		return git.GitStatusStateValues.Error, nil
	default:
		return "", errors.New("can't convert to azure devops state")
	}
}
