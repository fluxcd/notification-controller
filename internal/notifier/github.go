/*
Copyright 2020 The Flux CD contributors.

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
	"net/url"
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
	owner, repo, err := parseGithubAddress(addr)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &GitHub{
		Owner:  owner,
		Repo:   repo,
		Client: client,
	}, nil
}

// Post Github commit status
func (g *GitHub) Post(event recorder.Event) error {
	revString, ok := event.Metadata["revision"]
	if !ok {
		return errors.New("Missing revision metadata")
	}
	rev, err := parseRevision(revString)
	if err != nil {
		return err
	}
	state, err := toGitHubState(event.Severity)
	if err != nil {
		return err
	}

	githubCtx := fmt.Sprintf("%v/%v/%v", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
	desc := strings.Join(split(event.Reason), " ")
	status := &github.RepoStatus{
		State:       &state,
		Context:     &githubCtx,
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

func parseGithubAddress(addr string) (string, string, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", nil
	}

	comp := strings.Split(u.Path, "/")
	if len(comp) < 3 {
		return "", "", fmt.Errorf("Not enough components in path %v", u.Path)
	}

	return comp[1], comp[2], nil
}

func parseRevision(rev string) (string, error) {
	comp := strings.Split(rev, "/")
	if len(comp) < 2 {
		return "", errors.New("Revision string format incorrect")
	}

	return comp[1], nil
}

func toGitHubState(severity string) (string, error) {
	switch severity {
	case "info":
		return "success", nil
	case "error":
		return "failure", nil
	default:
		return "", errors.New("Can't convert to GitHub state")
	}
}
