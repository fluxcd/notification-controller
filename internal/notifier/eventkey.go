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
	"crypto/sha256"
	"fmt"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

// EventKey generates a unique key identifying an event. The key is calculated
// by concatenating specific event attributes and hashing them using SHA-256,
// and is returned as a hex-encoded string. It is the identity of an event
// across the controller: the event server uses it to rate limit duplicate
// events and notifiers may use it as an idempotency token.
//
// The event attributes are prefixed with an identifier to avoid collisions
// between different event attributes.
func EventKey(event *eventv1.Event) string {
	comps := []string{
		"event",
		"name=" + event.InvolvedObject.Name,
		"namespace=" + event.InvolvedObject.Namespace,
		"kind=" + event.InvolvedObject.Kind,
		"message=" + event.Message,
	}

	objectGroup := event.InvolvedObject.GetObjectKind().GroupVersionKind().Group

	originRevisionKey := fmt.Sprintf("%s/%s", objectGroup, eventv1.MetaOriginRevisionKey)
	originRevision, ok := event.Metadata[originRevisionKey]
	if ok {
		comps = append(comps, "originRevision="+originRevision)
	}

	revisionKey := fmt.Sprintf("%s/%s", objectGroup, eventv1.MetaRevisionKey)
	revision, ok := event.Metadata[revisionKey]
	if ok {
		comps = append(comps, "revision="+revision)
	}

	tokenKey := fmt.Sprintf("%s/%s", objectGroup, eventv1.MetaTokenKey)
	token, ok := event.Metadata[tokenKey]
	if ok {
		comps = append(comps, "token="+token)
	}

	key := strings.Join(comps, "/")
	digest := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", digest)
}
