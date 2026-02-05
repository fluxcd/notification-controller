/*
Copyright 2026 The Flux authors

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

package server

import (
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"

	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
)

// isChangeRequestCommentProvider returns true if the provider type is a
// change request comment provider.
func isChangeRequestCommentProvider(providerType string) bool {
	return providerType == apiv1beta3.GitHubPullRequestCommentProvider ||
		providerType == apiv1beta3.GitLabMergeRequestCommentProvider
}

// hasChangeRequestKey returns true if the event has the change request key in its metadata.
func hasChangeRequestKey(event *eventv1.Event) bool {
	return event.Metadata[eventv1.Group+"/"+eventv1.MetaChangeRequestKey] != ""
}
