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
	"errors"

	"github.com/fluxcd/pkg/recorder"
	"github.com/xanzy/go-gitlab"
)

type GitLab struct {
	Id     string
	Client *gitlab.Client
}

func NewGitLab(addr string, token string) (*GitLab, error) {
	if len(token) == 0 {
		return nil, errors.New("GitLab token  cannot be empty")
	}

	host, id, err := parseGitAddress(addr)
	if err != nil {
		return nil, err
	}

	opt := gitlab.WithBaseURL(host)
	client, err := gitlab.NewClient(token, opt)
	if err != nil {
		return nil, err
	}

	gitlab := &GitLab{
		Id:     id,
		Client: client,
	}

	return gitlab, nil
}

// Post GitLab commit status
func (g *GitLab) Post(event recorder.Event) error {
	revString, ok := event.Metadata["revision"]
	if !ok {
		return errors.New("Missing revision metadata")
	}
	rev, err := parseRevision(revString)
	if err != nil {
		return err
	}
	state, err := toGitLabState(event.Severity)
	if err != nil {
		return err
	}

	name, desc := formatNameAndDescription(event)
	options := &gitlab.SetCommitStatusOptions{
		Name:        &name,
		Description: &desc,
		State:       state,
	}

	_, _, err = g.Client.Commits.SetCommitStatus(g.Id, rev, options)
	if err != nil {
		return err
	}

	return nil
}

func toGitLabState(severity string) (gitlab.BuildStateValue, error) {
	switch severity {
	case "info":
		return gitlab.Success, nil
	case "error":
		return gitlab.Failed, nil
	default:
		return "", errors.New("Can't convert to GitLab state")
	}
}
