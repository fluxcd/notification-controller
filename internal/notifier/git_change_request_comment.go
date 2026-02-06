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

package notifier

import (
	"fmt"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

// changeRequestComment contains shared logic for change request comment providers
// (GitHub Pull Request Comment, GitLab Merge Request Comment, etc.).
type changeRequestComment struct {
	ProviderUID      string
	CommentKeyPrefix string
}

// generateCommentKey generates a unique comment key based on the provider UID,
// involved object kind, namespace, and name.
func (c *changeRequestComment) generateCommentKey(event *eventv1.Event) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		c.ProviderUID,
		event.InvolvedObject.Kind,
		event.InvolvedObject.Namespace,
		event.InvolvedObject.Name)
}

// formatCommentKeyMarker formats the comment key marker that is used to identify comments
// created by this notifier for a specific event. It is included in the comment body as an
// HTML comment, so it is not visible in the UI but can be used to find and update existing
// comments for the same event. The marker includes the generated comment key.
func (c *changeRequestComment) formatCommentKeyMarker(event *eventv1.Event) string {
	return fmt.Sprintf("<!-- %s:%s -->", c.CommentKeyPrefix, c.generateCommentKey(event))
}

// formatCommentBody formats the body of the change request comment based on the event data.
func (c *changeRequestComment) formatCommentBody(event *eventv1.Event) string {
	marker := c.formatCommentKeyMarker(event)
	body := formatMarkdownPost(event)
	return fmt.Sprintf("%s\n\n%s", marker, body)
}
