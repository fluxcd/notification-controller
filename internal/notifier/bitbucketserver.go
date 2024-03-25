/*
Copyright 2023 The Flux authors

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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/hashicorp/go-retryablehttp"
)

// BitbucketServer is a notifier for BitBucket Server and Data Center.
type BitbucketServer struct {
	ProjectKey      string
	RepositorySlug  string
	ProviderUID     string
	ProviderAddress string
	Host            string
	ContextPath     string
	Username        string
	Password        string
	Token           string
	Client          *retryablehttp.Client
}

const (
	bbServerEndPointTmpl              = "%[1]s/rest/api/latest/projects/%[2]s/repos/%[3]s/commits/%[4]s/builds"
	bbServerGetBuildStatusQueryString = "key"
)

type bbServerBuildStatus struct {
	Name        string `json:"name,omitempty"`
	Key         string `json:"key,omitempty"`
	Parent      string `json:"parent,omitempty"`
	State       string `json:"state,omitempty"`
	Ref         string `json:"ref,omitempty"`
	BuildNumber string `json:"buildNumber,omitempty"`
	Description string `json:"description,omitempty"`
	Duration    int64  `json:"duration,omitempty"`
	UpdatedDate int64  `json:"updatedDate,omitempty"`
	CreatedDate int64  `json:"createdDate,omitempty"`
	Url         string `json:"url,omitempty"`
}

type bbServerBuildStatusSetRequest struct {
	BuildNumber string `json:"buildNumber,omitempty"`
	Description string `json:"description,omitempty"`
	Duration    int64  `json:"duration,omitempty"`
	Key         string `json:"key"`
	LastUpdated int64  `json:"lastUpdated,omitempty"`
	Name        string `json:"name,omitempty"`
	Parent      string `json:"parent,omitempty"`
	Ref         string `json:"ref,omitempty"`
	State       string `json:"state"`
	Url         string `json:"url"`
}

// NewBitbucketServer creates and returns a new BitbucketServer notifier.
func NewBitbucketServer(providerUID string, addr string, token string, certPool *x509.CertPool, username string, password string) (*BitbucketServer, error) {
	hst, cntxtPath, id, err := parseBitbucketServerGitAddress(addr)
	if err != nil {
		return nil, err
	}

	comp := strings.Split(id, "/")
	if len(comp) != 2 {
		return nil, fmt.Errorf("invalid repository id %q", id)
	}
	projectkey := comp[0]
	reposlug := comp[1]

	httpClient := retryablehttp.NewClient()
	if certPool != nil {
		httpClient.HTTPClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		}
	}

	httpClient.HTTPClient.Timeout = 15 * time.Second
	httpClient.RetryWaitMin = 2 * time.Second
	httpClient.RetryWaitMax = 30 * time.Second
	httpClient.RetryMax = 4
	httpClient.Logger = nil

	if len(token) == 0 && (len(username) == 0 || len(password) == 0) {
		return nil, errors.New("invalid credentials, expected to be one of username/password or API Token")
	}

	return &BitbucketServer{
		ProjectKey:      projectkey,
		RepositorySlug:  reposlug,
		ProviderUID:     providerUID,
		Host:            hst,
		ProviderAddress: addr,
		ContextPath:     cntxtPath,
		Token:           token,
		Username:        username,
		Password:        password,
		Client:          httpClient,
	}, nil
}

// Post Bitbucket Server build status
func (b BitbucketServer) Post(ctx context.Context, event eventv1.Event) error {
	// Skip progressing events
	if event.HasReason(meta.ProgressingReason) {
		return nil
	}
	revString, ok := event.Metadata[eventv1.MetaRevisionKey]
	if !ok {
		return errors.New("missing revision metadata")
	}
	rev, err := parseRevision(revString)
	if err != nil {
		return fmt.Errorf("could not parse revision: %w", err)
	}
	state, err := b.state(event.Severity)
	if err != nil {
		return fmt.Errorf("couldn't convert to bitbucket server state: %w", err)
	}

	name, desc := formatNameAndDescription(event)
	name = name + " [" + desc + "]" //Bitbucket server displays this data on browser. Thus adding description here.
	id := generateCommitStatusID(b.ProviderUID, event)
	// key has a limitation of 40 characters in bitbucket api
	key := sha1String(id)

	u := b.Host + b.createApiPath(rev)
	dupe, err := b.duplicateBitbucketServerStatus(ctx, state, name, desc, key, u)
	if err != nil {
		return fmt.Errorf("could not get existing commit status: %w", err)
	}

	if !dupe {
		_, err = b.postBuildStatus(ctx, state, name, desc, key, u)
		if err != nil {
			return fmt.Errorf("could not post build status: %w", err)
		}
	}

	return nil
}

func (b BitbucketServer) state(severity string) (string, error) {
	switch severity {
	case eventv1.EventSeverityInfo:
		return "SUCCESSFUL", nil
	case eventv1.EventSeverityError:
		return "FAILED", nil
	default:
		return "", errors.New("bitbucket server state generated on info or error events only")
	}
}

func (b BitbucketServer) duplicateBitbucketServerStatus(ctx context.Context, state, name, desc, key, u string) (bool, error) {
	// Prepare request object
	req, err := b.prepareCommonRequest(ctx, u, nil, http.MethodGet)
	if err != nil {
		return false, fmt.Errorf("could not check duplicate commit status: %w", err)
	}

	// Set query string
	q := url.Values{}
	q.Add(bbServerGetBuildStatusQueryString, key)
	req.URL.RawQuery = q.Encode()

	// Make a GET call
	d, err := b.Client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed api call to check duplicate commit status: %w", err)
	}
	if d != nil && isError(d) && d.StatusCode != http.StatusNotFound {
		defer d.Body.Close()
		return false, fmt.Errorf("failed api call to check duplicate commit status: %d - %s", d.StatusCode, http.StatusText(d.StatusCode))
	}
	defer d.Body.Close()

	if d.StatusCode == http.StatusOK {
		bd, err := io.ReadAll(d.Body)
		if err != nil {
			return false, fmt.Errorf("could not read response body for duplicate commit status: %w", err)
		}
		var existingCommitStatus bbServerBuildStatus
		err = json.Unmarshal(bd, &existingCommitStatus)
		if err != nil {
			return false, fmt.Errorf("could not unmarshal json response body for duplicate commit status: %w", err)
		}
		// Do not post duplicate build status
		if existingCommitStatus.Key == key && existingCommitStatus.State == state && existingCommitStatus.Description == desc && existingCommitStatus.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (b BitbucketServer) postBuildStatus(ctx context.Context, state, name, desc, key, url string) (*http.Response, error) {
	//Prepare json body
	j := &bbServerBuildStatusSetRequest{
		Key:         key,
		State:       state,
		Url:         b.ProviderAddress,
		Description: desc,
		Name:        name,
	}
	p := new(bytes.Buffer)
	err := json.NewEncoder(p).Encode(j)
	if err != nil {
		return nil, fmt.Errorf("failed preparing request for post build commit status, could not encode request body to json: %w", err)
	}

	//Prepare request
	req, err := b.prepareCommonRequest(ctx, url, p, http.MethodPost)
	if err != nil {
		return nil, fmt.Errorf("failed preparing request for post build commit status: %w", err)
	}

	// Add Content type header
	req.Header.Add("Content-Type", "application/json")

	// Make a POST call
	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post build commit status: %w", err)
	}
	// Note: A non-2xx status code doesn't cause an error: https://pkg.go.dev/net/http#Client.Do
	if isError(resp) {
		defer resp.Body.Close()
		return nil, fmt.Errorf("could not post build commit status: %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	defer resp.Body.Close()
	return resp, nil
}

func (b BitbucketServer) createApiPath(rev string) string {
	return fmt.Sprintf(bbServerEndPointTmpl, b.ContextPath, b.ProjectKey, b.RepositorySlug, rev)
}

func parseBitbucketServerGitAddress(s string) (string, string, string, error) {
	scheme := strings.Split(s, ":")[0]
	if scheme != "http" && scheme != "https" {
		return "", "", "", fmt.Errorf("could not parse git address: unsupported scheme type in address: %s. Must be http or https", scheme)
	}

	host, idWithContext, err := parseGitAddress(s)
	if err != nil {
		return "", "", "", fmt.Errorf("could not parse git address: %w", err)
	}

	idWithContext = "/" + idWithContext

	// /scm/ is always part of http/https clone urls : https://community.atlassian.com/t5/Bitbucket-questions/remote-url-in-Bitbucket-server-what-does-scm-represent-is-it/qaq-p/2060987
	lastIndex := strings.LastIndex(idWithContext, "/scm/")
	if lastIndex < 0 {
		return "", "", "", fmt.Errorf("could not parse git address: supplied provider address is not http(s) git clone url")
	}

	// Handle context scenarios --> https://confluence.atlassian.com/bitbucketserver/change-bitbucket-s-context-path-776640153.html
	cntxtPath := idWithContext[:lastIndex] // Context path is anything that comes before last /scm/

	id := idWithContext[lastIndex+5:] // Remove last `/scm/` from id as it's is not used in API calls

	return host, cntxtPath, id, nil
}

func (b BitbucketServer) prepareCommonRequest(ctx context.Context, path string, body io.Reader, method string) (*retryablehttp.Request, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, fmt.Errorf("could not prepare request: %w", err)
	}

	if b.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.Token)
	} else {
		req.Header.Add("Authorization", "Basic "+basicAuth(b.Username, b.Password))
	}
	req.Header.Add("x-atlassian-token", "no-check")
	req.Header.Add("x-requested-with", "XMLHttpRequest")

	return req, nil
}

// isError method returns true if HTTP status `code >= 400` otherwise false.
func isError(r *http.Response) bool {
	return r.StatusCode > 399
}
