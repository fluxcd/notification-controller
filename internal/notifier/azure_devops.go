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
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
	"sigs.k8s.io/controller-runtime/pkg/client"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/cache"
)

const genre string = "fluxcd"

type azureDevOpsClient interface {
	CreateCommitStatus(context.Context, git.CreateCommitStatusArgs) (*git.GitStatus, error)
	GetStatuses(context.Context, git.GetStatusesArgs) (*[]git.GitStatus, error)
}

// AzureDevOps is an Azure DevOps notifier.
type AzureDevOps struct {
	Project      string
	Repo         string
	CommitStatus string
	Client       azureDevOpsClient
}

// NewAzureDevOps creates and returns a new AzureDevOps notifier.
func NewAzureDevOps(ctx context.Context, commitStatus string, addr string, token string,
	certPool *x509.CertPool, proxy, serviceAccountName, providerName, providerNamespace string,
	tokenClient client.Client, tokenCache *cache.TokenCache) (*AzureDevOps, error) {
	var err error

	if len(token) == 0 {
		// if token doesn't exist, try to create a new token using managed identity
		token, err = newManagedIdentityToken(ctx, proxy, serviceAccountName, providerName, providerNamespace, azure.ScopeDevOps, tokenClient, tokenCache)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire azure devops token: %w", err)
		}
	}

	host, id, err := parseGitAddress(addr)
	if err != nil {
		return nil, err
	}

	// this should never happen
	if commitStatus == "" {
		return nil, errors.New("commit status cannot be empty")
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
	if certPool != nil {
		connection.TlsConfig = &tls.Config{
			RootCAs: certPool,
		}
	}
	client := connection.GetClientByUrl(orgURL)
	gitClient := &git.ClientImpl{
		Client: *client,
	}
	return &AzureDevOps{
		Project:      proj,
		Repo:         repo,
		CommitStatus: commitStatus,
		Client:       gitClient,
	}, nil
}

// Post Azure DevOps commit status
func (a AzureDevOps) Post(ctx context.Context, event eventv1.Event) error {
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
	state, err := toAzureDevOpsState(event.Severity)
	if err != nil {
		return err
	}

	// Check if the exact status is already set
	g := commitStatusGenre(event)

	_, desc := formatNameAndDescription(event)
	id := a.CommitStatus
	createArgs := git.CreateCommitStatusArgs{
		Project:      &a.Project,
		RepositoryId: &a.Repo,
		CommitId:     &rev,
		GitCommitStatusToCreate: &git.GitStatus{
			Description: &desc,
			State:       &state,
			Context: &git.GitStatusContext{
				Genre: &g,
				Name:  &id,
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
		return fmt.Errorf("could not list commit statuses: %w", err)
	}

	if duplicateAzureDevOpsStatus(statuses, createArgs.GitCommitStatusToCreate) {
		return nil
	}

	// Create a new status
	_, err = a.Client.CreateCommitStatus(ctx, createArgs)
	if err != nil {
		return fmt.Errorf("could not create commit status: %w", err)
	}
	return nil
}

func toAzureDevOpsState(severity string) (git.GitStatusState, error) {
	switch severity {
	case eventv1.EventSeverityInfo:
		return git.GitStatusStateValues.Succeeded, nil
	case eventv1.EventSeverityError:
		return git.GitStatusStateValues.Error, nil
	default:
		return "", errors.New("can't convert to azure devops state")
	}
}

// duplicateStatus return true if the latest status
// with a matching context has the same state and description
func duplicateAzureDevOpsStatus(statuses *[]git.GitStatus, status *git.GitStatus) bool {
	if status == nil || status.Context == nil || statuses == nil {
		return false
	}

	for _, s := range *statuses {
		if s.Context == nil || s.Context.Name == nil || s.Context.Genre == nil ||
			s.State == nil || s.Description == nil {
			continue
		}

		if *s.Context.Name == *status.Context.Name && *s.Context.Genre == *status.Context.Genre {
			if *s.State == *status.State && *s.Description == *status.Description {
				return true
			}

			return false
		}
	}

	return false
}

func commitStatusGenre(event eventv1.Event) string {
	summary, ok := event.Metadata["summary"]
	if ok {
		return fmt.Sprintf("%s:%s", genre, summary)
	}

	return genre
}
