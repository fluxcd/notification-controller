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
	"encoding/json"
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

type GitHubDispatch struct {
	Owner  string
	Repo   string
	Client *github.Client
}

func NewGitHubDispatch(addr string, token string, certPool *x509.CertPool) (*GitHubDispatch, error) {
	if strings.Contains(addr, "http://127.0.0.1") {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(context.Background(), ts)
		client := github.NewClient(tc)
		return &GitHubDispatch{
			Owner:  "foo",
			Repo:   "bar",
			Client: client,
		}, nil
	}

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

	return &GitHubDispatch{
		Owner:  comp[0],
		Repo:   comp[1],
		Client: client,
	}, nil
}

// Post GitHub Repository Dispatch webhook
func (g *GitHubDispatch) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	kustomizationName := fmt.Sprintf("%s/%s.%s",
		event.InvolvedObject.Kind, event.InvolvedObject.Name, event.InvolvedObject.Namespace)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("Failed to marshal object into json: %w", err)
	}
	eventDataRaw := json.RawMessage(eventData)

	opts := github.DispatchRequestOptions{
		EventType:     kustomizationName,
		ClientPayload: &eventDataRaw,
	}
	_, _, err = g.Client.Repositories.Dispatch(ctx, g.Owner, g.Repo, opts)

	if err != nil {
		return fmt.Errorf("Could not send github repository dispatch webhook: %v", err)
	}

	return nil
}
