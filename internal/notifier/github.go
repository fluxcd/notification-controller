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
	"net/url"
	"strings"
	"time"

	"github.com/fluxcd/pkg/runtime/events"

	"github.com/google/go-github/v41/github"
	"golang.org/x/oauth2"
)

type GitHub struct {
	Owner  string
	Repo   string
	Client *github.Client
}

func NewGitHub(addr string, token string, certPool *x509.CertPool) (*GitHub, error) {
	if len(token) == 0 {
		return nil, errors.New("github token cannot be empty")
	}

	host, id, err := parseGitAddress(addr)
	if err != nil {
		return nil, err
	}

	baseUrl, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	comp := strings.Split(id, "/")
	if len(comp) != 2 {
		return nil, fmt.Errorf("invalid repository id %q", id)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)
	if baseUrl.Host != "github.com" {
		if certPool != nil {
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: certPool,
				},
			}
			hc := &http.Client{Transport: tr}
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, hc)
			tc = oauth2.NewClient(ctx, ts)
		}

		client, err = github.NewEnterpriseClient(host, host, tc)
		if err != nil {
			return nil, fmt.Errorf("could not create enterprise GitHub client: %v", err)
		}
	}

	return &GitHub{
		Owner:  comp[0],
		Repo:   comp[1],
		Client: client,
	}, nil
}

// Post Github commit status
func (g *GitHub) Post(event events.Event) error {
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
	state, err := toGitHubState(event.Severity)
	if err != nil {
		return err
	}
	name, desc := formatNameAndDescription(event)

	status := &github.RepoStatus{
		State:       &state,
		Context:     &name,
		Description: &desc,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opts := &github.ListOptions{PerPage: 50}
	statuses, _, err := g.Client.Repositories.ListStatuses(ctx, g.Owner, g.Repo, rev, opts)
	if err != nil {
		return fmt.Errorf("could not list commit statuses: %v", err)
	}
	if duplicateGithubStatus(statuses, status) {
		return nil
	}

	_, _, err = g.Client.Repositories.CreateStatus(ctx, g.Owner, g.Repo, rev, status)
	if err != nil {
		return fmt.Errorf("could not create commit status: %v", err)
	}

	return nil
}

func toGitHubState(severity string) (string, error) {
	switch severity {
	case events.EventSeverityInfo:
		return "success", nil
	case events.EventSeverityError:
		return "failure", nil
	default:
		return "", errors.New("can't convert to github state")
	}
}

// duplicateStatus return true if the latest status
// with a matching context has the same state and description
func duplicateGithubStatus(statuses []*github.RepoStatus, status *github.RepoStatus) bool {
	for _, s := range statuses {
		if *s.Context == *status.Context {
			if *s.State == *status.State && *s.Description == *status.Description {
				return true
			}

			return false
		}
	}

	return false
}
