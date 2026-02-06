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
	"slices"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

// formatMarkdownPost formats the event for Markdown rendering engines.
func formatMarkdownPost(event *eventv1.Event) string {
	// Get emoji based on severity
	var severityEmoji string
	if event.Severity == eventv1.EventSeverityError {
		severityEmoji = "⚠️"
	} else {
		severityEmoji = "ℹ️"
	}

	// Format object identifier
	objectID := fmt.Sprintf("%s/%s/%s",
		event.InvolvedObject.Kind,
		event.InvolvedObject.Namespace,
		event.InvolvedObject.Name)

	// Build metadata section
	keys := make([]string, 0, len(event.Metadata))
	for k := range event.Metadata {
		if k != eventv1.MetaChangeRequestKey {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	var metadataLines strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&metadataLines, "* `%s`: %s\n", key, event.Metadata[key])
	}

	// Format the comment body
	return fmt.Sprintf("## Flux Status\n\n%s %s\n\n%s\n\nMetadata:\n%s",
		severityEmoji, objectID, event.Message, metadataLines.String())
}
