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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"net/http"
	"net/http/httptest"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewBitbucketServerBasicNoContext(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/projectfoo/repobar.git", "", nil, "dummyuser", "testpassword")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(b.Username).To(Equal("dummyuser"))
	g.Expect(b.Password).To(Equal("testpassword"))
	g.Expect(b.Url.Scheme).To(Equal("https"))
	g.Expect(b.Url.Host).To(Equal("example.com:7990"))
}

func TestNewBitbucketServerBasicWithContext(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/context/scm/projectfoo/repobar.git", "", nil, "dummyuser", "testpassword")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(b.Username).To(Equal("dummyuser"))
	g.Expect(b.Password).To(Equal("testpassword"))
	g.Expect(b.Url.Scheme).To(Equal("https"))
	g.Expect(b.Url.Host).To(Equal("example.com:7990"))
}

func TestBitbucketServerApiPathNoContext(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/projectfoo/repobar.git", "", nil, "dummyuser", "testpassword")
	g.Expect(err).ToNot(HaveOccurred())
	u := b.Url.JoinPath(b.createBuildPath("00151b98e303e19610378e6f1c49e31e5e80cd3b")).String()
	g.Expect(u).To(Equal("https://example.com:7990/rest/api/latest/projects/projectfoo/repos/repobar/commits/00151b98e303e19610378e6f1c49e31e5e80cd3b/builds"))
}

func TestBitbucketServerApiPathOneWordContext(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/context1/scm/projectfoo/repobar.git", "", nil, "dummyuser", "testpassword")
	g.Expect(err).ToNot(HaveOccurred())
	u := b.Url.JoinPath(b.createBuildPath("00151b98e303e19610378e6f1c49e31e5e80cd3b")).String()
	g.Expect(u).To(Equal("https://example.com:7990/context1/rest/api/latest/projects/projectfoo/repos/repobar/commits/00151b98e303e19610378e6f1c49e31e5e80cd3b/builds"))
}

func TestBitbucketServerApiPathMultipleWordContext(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/context1/context2/context3/scm/projectfoo/repobar.git", "", nil, "dummyuser", "testpassword")
	g.Expect(err).ToNot(HaveOccurred())
	u := b.Url.JoinPath(b.createBuildPath("00151b98e303e19610378e6f1c49e31e5e80cd3b")).String()
	g.Expect(u).To(Equal("https://example.com:7990/context1/context2/context3/rest/api/latest/projects/projectfoo/repos/repobar/commits/00151b98e303e19610378e6f1c49e31e5e80cd3b/builds"))
}

func TestBitbucketServerApiPathOneWordScmInContext(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/scm/projectfoo/repobar.git", "", nil, "dummyuser", "testpassword")
	g.Expect(err).ToNot(HaveOccurred())
	u := b.Url.JoinPath(b.createBuildPath("00151b98e303e19610378e6f1c49e31e5e80cd3b")).String()
	g.Expect(u).To(Equal("https://example.com:7990/scm/rest/api/latest/projects/projectfoo/repos/repobar/commits/00151b98e303e19610378e6f1c49e31e5e80cd3b/builds"))
}

func TestBitbucketServerApiPathMultipleWordScmInContext(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/context2/scm/scm/projectfoo/repobar.git", "", nil, "dummyuser", "testpassword")
	g.Expect(err).ToNot(HaveOccurred())
	u := b.Url.JoinPath(b.createBuildPath("00151b98e303e19610378e6f1c49e31e5e80cd3b")).String()
	g.Expect(u).To(Equal("https://example.com:7990/scm/context2/scm/rest/api/latest/projects/projectfoo/repos/repobar/commits/00151b98e303e19610378e6f1c49e31e5e80cd3b/builds"))
}

func TestBitbucketServerApiPathScmAlreadyRemovedInInput(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/context1/context2/context3/projectfoo/repobar.git", "", nil, "dummyuser", "testpassword")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("could not parse git address: supplied provider address is not http(s) git clone url"))
}

func TestBitbucketServerSshAddress(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "ssh://git@mybitbucket:2222/ap/fluxcd-sandbox.git", "", nil, "", "")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("could not parse git address: unsupported scheme type in address: ssh. Must be http or https"))
}

func TestNewBitbucketServerToken(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/projectfoo/repobar.git", "BBDC-ODIxODYxMzIyNzUyOttorMjO059P2rYTb6EH7mP", nil, "", "")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(b.Token).To(Equal("BBDC-ODIxODYxMzIyNzUyOttorMjO059P2rYTb6EH7mP"))
}

func TestNewBitbucketServerInvalidCreds(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/projectfoo/repobar.git", "", nil, "", "")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("invalid credentials, expected to be one of username/password or API Token"))
}

func TestNewBitbucketServerInvalidRepo(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/projectfoo/repobar/invalid.git", "BBDC-ODIxODYxMzIyNzUyOttorMjO059P2rYTb6EH7mP", nil, "", "")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("could not parse git address: invalid repository id \"projectfoo/repobar/invalid\""))
}

func TestPostBitbucketServerMissingRevision(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/projectfoo/repobar.git", "BBDC-ODIxODYxMzIyNzUyOttorMjO059P2rYTb6EH7mP", nil, "", "")
	g.Expect(err).ToNot(HaveOccurred())

	//Validate missing revision
	err = b.Post(context.TODO(), generateTestEventKustomization("info", map[string]string{
		"dummybadrevision": "bad",
	}))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("missing revision metadata"))
}

func TestNewBitbucketServerEmptyCommitStatus(t *testing.T) {
	g := NewWithT(t)
	_, err := NewBitbucketServer("", "https://example.com:7990/scm/projectfoo/repobar.git", "BBDC-ODIxODYxMzIyNzUyOttorMjO059P2rYTb6EH7mP", nil, "", "")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("commit status cannot be empty"))
}

func TestPostBitbucketServerBadCommitHash(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/projectfoo/repobar.git", "BBDC-ODIxODYxMzIyNzUyOttorMjO059P2rYTb6EH7mP", nil, "", "")
	g.Expect(err).ToNot(HaveOccurred())

	//Validate extract commit hash
	err = b.Post(context.TODO(), generateTestEventKustomization("info", map[string]string{
		eventv1.MetaRevisionKey: "badhash",
	}))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("could not parse revision: failed to extract commit hash from 'badhash' revision"))

}

func TestPostBitbucketServerBadBitbucketState(t *testing.T) {
	g := NewWithT(t)
	b, err := NewBitbucketServer("kustomization/gitops-system/0c9c2e41", "https://example.com:7990/scm/projectfoo/repobar.git", "BBDC-ODIxODYxMzIyNzUyOttorMjO059P2rYTb6EH7mP", nil, "", "")
	g.Expect(err).ToNot(HaveOccurred())

	//Validate conversion to bitbucket state
	err = b.Post(context.TODO(), generateTestEventKustomization("badserveritystate", map[string]string{
		eventv1.MetaRevisionKey: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
	}))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("couldn't convert to bitbucket server state: bitbucket server state generated on info or error events only"))

}

func generateTestEventKustomization(severity string, metadata map[string]string) eventv1.Event {
	return eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Kustomization",
			Namespace: "flux-system",
			Name:      "hello-world",
		},
		Severity:            severity,
		Timestamp:           metav1.Now(),
		Message:             "message",
		Reason:              "reason",
		Metadata:            metadata,
		ReportingController: "kustomize-controller",
		ReportingInstance:   "kustomize-controller-xyz",
	}
}

func TestBitBucketServerPostValidateRequest(t *testing.T) {
	tests := []struct {
		name           string
		errorString    string
		testFailReason string
		headers        map[string]string
		username       string
		password       string
		token          string
		event          eventv1.Event
		commitStatus   string
		key            string
		uriHash        string
	}{
		{
			name:         "Validate Token Auth ",
			token:        "goodtoken",
			commitStatus: "kustomization/gitops-system/0c9c2e41",
			headers: map[string]string{
				"Authorization":     "Bearer goodtoken",
				"x-atlassian-token": "no-check",
				"x-requested-with":  "XMLHttpRequest",
			},
			event: generateTestEventKustomization("info", map[string]string{
				eventv1.MetaRevisionKey: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			}),
			key:     sha1String("kustomization/gitops-system/0c9c2e41"),
			uriHash: "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name:         "Event with origin revision",
			token:        "goodtoken",
			commitStatus: "kustomization/gitops-system/0c9c2e41",
			headers: map[string]string{
				"Authorization":     "Bearer goodtoken",
				"x-atlassian-token": "no-check",
				"x-requested-with":  "XMLHttpRequest",
			},
			event: generateTestEventKustomization("info", map[string]string{
				eventv1.MetaRevisionKey:       "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
				eventv1.MetaOriginRevisionKey: "main@sha1:e7c17dd8b8384bbc84b7e7385394cb7f48332b2d",
			}),
			key:     sha1String("kustomization/gitops-system/0c9c2e41"),
			uriHash: "e7c17dd8b8384bbc84b7e7385394cb7f48332b2d",
		},
		{
			name:         "Validate Basic Auth and Post State=Successful",
			username:     "hello",
			password:     "password",
			commitStatus: "kustomization/gitops-system/0c9c2e41",
			headers: map[string]string{
				"Authorization":     "Basic " + base64.StdEncoding.EncodeToString([]byte("hello"+":"+"password")),
				"x-atlassian-token": "no-check",
				"x-requested-with":  "XMLHttpRequest",
			},
			event: generateTestEventKustomization("info", map[string]string{
				eventv1.MetaRevisionKey: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			}),
			key:     sha1String("kustomization/gitops-system/0c9c2e41"),
			uriHash: "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name:         "Validate Post State=Failed",
			username:     "hello",
			password:     "password",
			commitStatus: "kustomization/gitops-system/0c9c2e41",
			headers: map[string]string{
				"Authorization":     "Basic " + base64.StdEncoding.EncodeToString([]byte("hello"+":"+"password")),
				"x-atlassian-token": "no-check",
				"x-requested-with":  "XMLHttpRequest",
			},
			event: generateTestEventKustomization("error", map[string]string{
				eventv1.MetaRevisionKey: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			}),
			key:     sha1String("kustomization/gitops-system/0c9c2e41"),
			uriHash: "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name:           "Fail if bad json response in existing commit status",
			testFailReason: "badjson",
			errorString:    "could not get existing commit status: could not unmarshal json response body for duplicate commit status: unexpected end of JSON input",
			username:       "hello",
			password:       "password",
			commitStatus:   "kustomization/gitops-system/0c9c2e41",
			headers: map[string]string{
				"Authorization":     "Basic " + base64.StdEncoding.EncodeToString([]byte("hello"+":"+"password")),
				"x-atlassian-token": "no-check",
				"x-requested-with":  "XMLHttpRequest",
			},
			event: generateTestEventKustomization("error", map[string]string{
				eventv1.MetaRevisionKey: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			}),
			key:     sha1String("kustomization/gitops-system/0c9c2e41"),
			uriHash: "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name:           "Fail if status code is non-200 in existing commit status",
			testFailReason: "badstatuscode",
			errorString:    "could not get existing commit status: failed api call to check duplicate commit status: 400 - Bad Request",
			username:       "hello",
			password:       "password",
			commitStatus:   "kustomization/gitops-system/0c9c2e41",
			headers: map[string]string{
				"Authorization":     "Basic " + base64.StdEncoding.EncodeToString([]byte("hello"+":"+"password")),
				"x-atlassian-token": "no-check",
				"x-requested-with":  "XMLHttpRequest",
			},
			event: generateTestEventKustomization("error", map[string]string{
				eventv1.MetaRevisionKey: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			}),
			key:     sha1String("kustomization/gitops-system/0c9c2e41"),
			uriHash: "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name:           "Bad post- Unauthorized",
			testFailReason: "badpost",
			errorString:    "could not post build status: could not post build commit status: 401 - Unauthorized",
			username:       "hello",
			password:       "password",
			commitStatus:   "kustomization/gitops-system/0c9c2e41",
			headers: map[string]string{
				"Authorization":     "Basic " + base64.StdEncoding.EncodeToString([]byte("hello"+":"+"password")),
				"x-atlassian-token": "no-check",
				"x-requested-with":  "XMLHttpRequest",
			},
			event: generateTestEventKustomization("error", map[string]string{
				eventv1.MetaRevisionKey: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			}),
			key:     sha1String("kustomization/gitops-system/0c9c2e41"),
			uriHash: "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name:         "Validate duplicate commit status successful match",
			username:     "hello",
			password:     "password",
			commitStatus: "kustomization/gitops-system/0c9c2e41",
			headers: map[string]string{
				"Authorization":     "Basic " + base64.StdEncoding.EncodeToString([]byte("hello"+":"+"password")),
				"x-atlassian-token": "no-check",
				"x-requested-with":  "XMLHttpRequest",
			},
			event: generateTestEventKustomization("info", map[string]string{
				eventv1.MetaRevisionKey: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			}),
			key:     sha1String("kustomization/gitops-system/0c9c2e41"),
			uriHash: "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				// Validate Headers
				for key, value := range tt.headers {
					g.Expect(r.Header.Get(key)).To(Equal(value))
				}

				// Validate URI
				path := fmt.Sprintf("/rest/api/latest/projects/projectfoo/repos/repobar/commits/%s/builds", tt.uriHash)
				g.Expect(r.URL.Path).To(Equal(path))

				// Validate Get Build Status call
				if r.Method == http.MethodGet {

					//Validate that this GET request has a query string with "key" as the query paraneter
					g.Expect(r.URL.Query().Get(bbServerGetBuildStatusQueryString)).To(Equal(tt.key))

					// Validate that this GET request has no body
					g.Expect(r.Body).To(Equal(http.NoBody))

					if tt.name == "Validate duplicate commit status successful match" {
						w.WriteHeader(http.StatusOK)
						w.Header().Add("Content-Type", "application/json")
						name, desc := formatNameAndDescription(tt.event)
						name = name + " [" + desc + "]"
						jsondata, _ := json.Marshal(&bbServerBuildStatus{
							Name:        name,
							Description: desc,
							Key:         sha1String(tt.commitStatus),
							State:       "SUCCESSFUL",
							Url:         "https://example.com:7990/scm/projectfoo/repobar.git",
						})
						w.Write(jsondata)
					}
					if tt.testFailReason == "badstatuscode" {
						w.WriteHeader(http.StatusBadRequest)
					} else if tt.testFailReason == "badjson" {
						w.WriteHeader(http.StatusOK)
						w.Header().Add("Content-Type", "application/json")
						//Do nothing here and an empty/null body will be returned
					} else {
						if tt.name != "Validate duplicate commit status successful match" {
							w.WriteHeader(http.StatusOK)
							w.Header().Add("Content-Type", "application/json")
							w.Write([]byte(`{
						"description": "reconciliation succeeded",
						"key": "TEST2",
						"state": "SUCCESSFUL",
						"name": "kustomization/helloworld-yaml-2-bitbucket-server [reconciliation succeeded]",
						"url": "https://example.com:7990/scm/projectfoo/repobar.git"
					}`))
						}
					}
				}

				// Validate Post BuildStatus call
				if r.Method == http.MethodPost {

					// Validate that this POST request has no query string
					g.Expect(len(r.URL.Query())).To(Equal(0))

					// Validate that this POST request has Content-Type: application/json header
					g.Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

					// Read json body of the request
					b, err := io.ReadAll(r.Body)
					g.Expect(err).ToNot(HaveOccurred())

					// Parse json request into Payload Request body struct
					var payload bbServerBuildStatusSetRequest
					err = json.Unmarshal(b, &payload)
					g.Expect(err).ToNot(HaveOccurred())

					// Validate Key
					g.Expect(payload.Key).To(Equal(tt.key))

					// Validate that state can be only SUCCESSFUL or FAILED
					if payload.State != "SUCCESSFUL" && payload.State != "FAILED" {
						g.Expect(payload.State).To(Or(Equal("SUCCESSFUL"), Equal("FAILED")))
					}

					// If severity of event is info, state should be SUCCESSFUL
					if tt.event.Severity == "info" {
						g.Expect(payload.State).To(Equal("SUCCESSFUL"))
					}

					// If severity of event is error, state should be FAILED
					if tt.event.Severity == "error" {
						g.Expect(payload.State).To(Equal("FAILED"))
					}

					// Validate description
					g.Expect(payload.Description).To(Equal("reason"))

					// Validate name(with description appended)
					g.Expect(payload.Name).To(Equal("kustomization/hello-world" + " [" + payload.Description + "]"))

					g.Expect(payload.Url).To(ContainSubstring("/scm/projectfoo/repobar.git"))

					if tt.testFailReason == "badpost" {
						w.WriteHeader(http.StatusUnauthorized)
					}

					// Sending a bad response here
					// This proves that the duplicate commit status is never posted
					if tt.name == "Validate duplicate commit status successful match" {
						w.WriteHeader(http.StatusUnauthorized)
					}
				}
			}))
			defer ts.Close()
			c, err := NewBitbucketServer(tt.commitStatus, ts.URL+"/scm/projectfoo/repobar.git", tt.token, nil, tt.username, tt.password)
			g.Expect(err).ToNot(HaveOccurred())
			err = c.Post(context.TODO(), tt.event)
			if tt.testFailReason == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.errorString))
			}
		})
	}
}
