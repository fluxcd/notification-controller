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
	"time"

	"github.com/fluxcd/pkg/recorder"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

type GitHub struct {
	Owner  string
	Repo   string
	Client *github.Client
}

func NewGitHub(addr string, token string) (*GitHub, error) {
	if len(token) == 0 {
		return nil, errors.New("github token cannot be empty")
	}

	_, id, err := parseGitAddress(addr)
	if err != nil {
		return nil, err
	}

	comp := strings.Split(id, "/")
	if len(comp) != 2 {
		return nil, fmt.Errorf("invalid repository id %q", id)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &GitHub{
		Owner:  comp[0],
		Repo:   comp[1],
		Client: client,
	}, nil
}

// Post Github commit status
func (g *GitHub) Post(event recorder.Event) error {
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
	_, _, err = g.Client.Repositories.CreateStatus(ctx, g.Owner, g.Repo, rev, status)
	if err != nil {
		return err
	}

	return nil
}

func toGitHubState(severity string) (string, error) {
	switch severity {
	case recorder.EventSeverityInfo:
		return "success", nil
	case recorder.EventSeverityError:
		return "failure", nil
	default:
		return "", errors.New("can't convert to github state")
	}
}
