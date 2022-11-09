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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"

	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

type GitHubDispatch struct {
	Owner  string
	Repo   string
	Client *github.Client
}

func NewGitHubDispatch(addr string, token string, certPool *x509.CertPool) (*GitHubDispatch, error) {
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

func NewGitHubAppDispatch(addr string, appID int64, installationID int64, privateKey []byte, certPool *x509.CertPool) (*GitHubDispatch, error) {
	if appID == 0 {
		return nil, errors.New("appID cannot be empty")
	}

	if installationID == 0 {
		return nil, errors.New("installationID cannot be empty")
	}

	if len(privateKey) == 0 {
		return nil, errors.New("privateKey cannot be empty")
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

	itr, err := ghinstallation.New(http.DefaultTransport,
		appID, installationID, privateKey)
	if err != nil {
		return nil, err
	}
	client := github.NewClient(&http.Client{Transport: itr})
	if baseUrl.Host != "github.com" {
		if certPool != nil {
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: certPool,
				},
			}

			itr, err := ghinstallation.New(tr, appID, installationID, privateKey)
			if err != nil {
				return nil, err
			}
			itr.BaseURL = host
		}
		client, err = github.NewEnterpriseClient(host, host, &http.Client{Transport: itr})
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
func (g *GitHubDispatch) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	eventType := fmt.Sprintf("%s/%s.%s",
		event.InvolvedObject.Kind, event.InvolvedObject.Name, event.InvolvedObject.Namespace)

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("Failed to marshal object into json: %w", err)
	}
	eventDataRaw := json.RawMessage(eventData)

	opts := github.DispatchRequestOptions{
		EventType:     eventType,
		ClientPayload: &eventDataRaw,
	}
	_, _, err = g.Client.Repositories.Dispatch(ctx, g.Owner, g.Repo, opts)

	if err != nil {
		return fmt.Errorf("Could not send github repository dispatch webhook: %v", err)
	}

	return nil
}
