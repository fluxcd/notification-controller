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
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestNewAzureDevOpsBasic(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	a, err := NewAzureDevOps(ctx, "kustomization/gitops-system/0c9c2e41", "https://dev.azure.com/foo/bar/_git/baz", "foo", nil, "", "", "", "", nil, nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(a.Project).To(Equal("bar"))
	g.Expect(a.Repo).To(Equal("baz"))
}

func TestNewAzureDevOpsInvalidUrl(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := NewAzureDevOps(ctx, "kustomization/gitops-system/0c9c2e41", "https://dev.azure.com/foo/bar/baz", "foo", nil, "", "", "", "", nil, nil)
	g.Expect(err).To(HaveOccurred())
}

func TestNewAzureDevOpsMissingToken(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := NewAzureDevOps(ctx, "kustomization/gitops-system/0c9c2e41", "https://dev.azure.com/foo/bar/baz", "", nil, "", "", "", "", nil, nil)
	g.Expect(err).To(HaveOccurred())
}

func TestNewAzureDevOpsEmptyCommitStatus(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := NewAzureDevOps(ctx, "", "https://dev.azure.com/foo/bar/_git/baz", "foo", nil, "", "", "", "", nil, nil)
	g.Expect(err).To(HaveOccurred())
}

func TestDuplicateAzureDevOpsStatus(t *testing.T) {
	g := NewWithT(t)

	var tests = []struct {
		ss  *[]git.GitStatus
		s   *git.GitStatus
		dup bool
	}{
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar"), true},
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Failed, "foo", "bar"), false},
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Succeeded, "baz", "bar"), false},
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Succeeded, "foo", "baz"), false},
		{&[]git.GitStatus{*azStatus(git.GitStatusStateValues.Succeeded, "baz", "bar"), *azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar")}, azStatus(git.GitStatusStateValues.Succeeded, "foo", "bar"), true},
	}

	for _, test := range tests {
		g.Expect(duplicateAzureDevOpsStatus(test.ss, test.s)).To(Equal(test.dup))
	}
}

func TestAzureDevOps_Post(t *testing.T) {
	strPtr := func(s string) *string {
		return &s
	}

	postTests := []struct {
		name  string
		event eventv1.Event
		want  git.CreateCommitStatusArgs
	}{
		{
			name: "event with no summary",
			event: eventv1.Event{
				Severity: eventv1.EventSeverityInfo,
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					eventv1.MetaRevisionKey: "main@sha1:69b59063470310ebbd88a9156325322a124e55a3",
				},
				Reason: "ApplySucceeded",
			},
			want: git.CreateCommitStatusArgs{
				CommitId:     strPtr("69b59063470310ebbd88a9156325322a124e55a3"),
				Project:      strPtr("bar"),
				RepositoryId: strPtr("baz"),
				GitCommitStatusToCreate: &git.GitStatus{
					Description: strPtr("apply succeeded"),
					State:       &git.GitStatusStateValues.Succeeded,
					Context: &git.GitStatusContext{
						Genre: strPtr("fluxcd"),
						Name:  strPtr("kustomization/gitops-system/0c9c2e41"),
					},
				},
			},
		},
		{
			name: "event with origin revision",
			event: eventv1.Event{
				Severity: eventv1.EventSeverityInfo,
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					eventv1.MetaRevisionKey:       "main@sha1:69b59063470310ebbd88a9156325322a124e55a3",
					eventv1.MetaOriginRevisionKey: "main@sha1:bd88a9156325322a124e55a369b59063470310eb",
				},
				Reason: "ApplySucceeded",
			},
			want: git.CreateCommitStatusArgs{
				CommitId:     strPtr("bd88a9156325322a124e55a369b59063470310eb"),
				Project:      strPtr("bar"),
				RepositoryId: strPtr("baz"),
				GitCommitStatusToCreate: &git.GitStatus{
					Description: strPtr("apply succeeded"),
					State:       &git.GitStatusStateValues.Succeeded,
					Context: &git.GitStatusContext{
						Genre: strPtr("fluxcd"),
						Name:  strPtr("kustomization/gitops-system/0c9c2e41"),
					},
				},
			},
		},
		{
			name: "event with summary",
			event: eventv1.Event{
				Severity: eventv1.EventSeverityInfo,
				InvolvedObject: corev1.ObjectReference{
					Kind: "Kustomization",
					Name: "gitops-system",
				},
				Metadata: map[string]string{
					eventv1.MetaRevisionKey: "main@sha1:69b59063470310ebbd88a9156325322a124e55a3",
					"summary":               "test summary",
				},
				Reason: "ApplySucceeded",
			},
			want: git.CreateCommitStatusArgs{
				CommitId:     strPtr("69b59063470310ebbd88a9156325322a124e55a3"),
				Project:      strPtr("bar"),
				RepositoryId: strPtr("baz"),
				GitCommitStatusToCreate: &git.GitStatus{
					Description: strPtr("apply succeeded"),
					State:       &git.GitStatusStateValues.Succeeded,
					Context: &git.GitStatusContext{
						Genre: strPtr("fluxcd:test summary"),
						Name:  strPtr("kustomization/gitops-system/0c9c2e41"),
					},
				},
			},
		},
	}

	for _, tt := range postTests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			a, err := NewAzureDevOps(ctx, "kustomization/gitops-system/0c9c2e41", "https://example.com/foo/bar/_git/baz", "foo", nil, "", "", "", "", nil, nil)
			fakeClient := &fakeDevOpsClient{}
			a.Client = fakeClient
			g := NewWithT(t)
			g.Expect(err).ToNot(HaveOccurred())

			err = a.Post(ctx, tt.event)
			g.Expect(err).ToNot(HaveOccurred())

			want := []git.CreateCommitStatusArgs{tt.want}
			g.Expect(fakeClient.created).To(Equal(want))
		})
	}
}

func azStatus(state git.GitStatusState, context string, description string) *git.GitStatus {
	genre := "fluxcd"
	return &git.GitStatus{
		Context: &git.GitStatusContext{
			Name:  &context,
			Genre: &genre,
		},
		Description: &description,
		State:       &state,
	}
}

type fakeDevOpsClient struct {
	created []git.CreateCommitStatusArgs
}

func (c *fakeDevOpsClient) CreateCommitStatus(ctx context.Context, args git.CreateCommitStatusArgs) (*git.GitStatus, error) {
	c.created = append(c.created, args)
	return nil, nil
}

func (c *fakeDevOpsClient) GetStatuses(context.Context, git.GetStatusesArgs) (*[]git.GitStatus, error) {
	return nil, nil
}
