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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ktrysmt/go-bitbucket"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
)

// Bitbucket is a Bitbucket Server notifier.
type Bitbucket struct {
	Owner        string
	Repo         string
	CommitStatus string
	Client       *bitbucket.Client
}

// NewBitbucket creates and returns a new Bitbucket notifier.
func NewBitbucket(commitStatus string, addr string, token string, tlsConfig *tls.Config) (*Bitbucket, error) {
	if len(token) == 0 {
		return nil, errors.New("bitbucket token cannot be empty")
	}

	// this should never happen
	if commitStatus == "" {
		return nil, errors.New("commit status cannot be empty")
	}

	_, id, err := parseGitAddress(addr)
	if err != nil {
		return nil, err
	}

	comp := strings.Split(token, ":")
	if len(comp) != 2 {
		return nil, errors.New("invalid token format, expected to be <user>:<password>")
	}
	username := comp[0]
	password := comp[1]

	comp = strings.Split(id, "/")
	if len(comp) != 2 {
		return nil, fmt.Errorf("invalid repository id %q", id)
	}
	owner := comp[0]
	repo := comp[1]

	// Support two authentication modes depending on secret content:
	// - If username == "x-token-auth" or "x-bitbucket-api-token-auth" the
	//   password is treated as an OAuth bearer token and we should use
	//   NewOAuthbearerToken.
	// - Otherwise use basic auth with <bitbucket_user_email>:<personal_api_token>.
	var client *bitbucket.Client
	if username == "x-token-auth" || username == "x-bitbucket-api-token-auth" {
		// password in this case is the bearer token
		client = bitbucket.NewOAuthbearerToken(password)
	} else {
		client = bitbucket.NewBasicAuth(username, password)
	}
	if tlsConfig != nil {
		tr := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		hc := &http.Client{Transport: tr}
		client.HttpClient = hc
	}

	return &Bitbucket{
		Owner:        owner,
		Repo:         repo,
		CommitStatus: commitStatus,
		Client:       client,
	}, nil
}

// Post Bitbucket commit status
func (b Bitbucket) Post(ctx context.Context, event eventv1.Event) error {
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
	state, err := toBitbucketState(event.Severity)
	if err != nil {
		return err
	}

	name, desc := formatNameAndDescription(event)
	id := b.CommitStatus
	// key has a limitation of 40 characters in bitbucket api
	key := sha1String(id)

	cmo := &bitbucket.CommitsOptions{
		Owner:    b.Owner,
		RepoSlug: b.Repo,
		Revision: rev,
	}
	cso := &bitbucket.CommitStatusOptions{
		State:       state,
		Key:         key,
		Name:        name,
		Description: desc,
		Url:         "https://bitbucket.org",
	}

	existingCommitStatus, err := b.Client.Repositories.Commits.GetCommitStatus(cmo, cso.Key)
	var statusErr *bitbucket.UnexpectedResponseStatusError
	if err != nil && !(errors.As(err, &statusErr) && strings.Contains(statusErr.Status, http.StatusText(http.StatusNotFound))) {
		return fmt.Errorf("could not get commit status: %v", err)
	}
	dupe, err := duplicateBitbucketStatus(existingCommitStatus, cso)
	if err != nil {
		return err
	}
	if dupe {
		return nil
	}
	_, err = b.Client.Repositories.Commits.CreateCommitStatus(cmo, cso)
	if err != nil {
		return err
	}

	return nil
}

func duplicateBitbucketStatus(statuses interface{}, status *bitbucket.CommitStatusOptions) (bool, error) {
	commitStatus := bitbucket.CommitStatusOptions{}
	b, err := json.Marshal(statuses)
	if err != nil {
		return false, err
	}
	err = json.Unmarshal(b, &commitStatus)
	if err != nil {
		return false, err
	}
	if commitStatus.State == status.State && commitStatus.Description == status.Description {
		return true, nil
	}
	return false, nil
}

func toBitbucketState(severity string) (string, error) {
	switch severity {
	case eventv1.EventSeverityInfo:
		return "SUCCESSFUL", nil
	case eventv1.EventSeverityError:
		return "FAILED", nil
	default:
		return "", errors.New("can't convert to bitbucket state")
	}
}
