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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestEventKey(t *testing.T) {
	g := NewWithT(t)

	base := func() eventv1.Event {
		return eventv1.Event{
			InvolvedObject: corev1.ObjectReference{
				APIVersion: "source.toolkit.fluxcd.io/v1",
				Kind:       "GitRepository",
				Namespace:  "flux-system",
				Name:       "podinfo",
			},
			Message: "stored artifact",
			Metadata: map[string]string{
				"source.toolkit.fluxcd.io/revision": "main@sha1:abc",
			},
		}
	}

	a, b := base(), base()
	g.Expect(EventKey(&a)).To(Equal(EventKey(&b)), "equal events must share a key")
	g.Expect(EventKey(&a)).To(HaveLen(64), "key is a hex-encoded SHA-256")

	// Timestamp and severity are not part of the identity.
	b.Severity = eventv1.EventSeverityError
	g.Expect(EventKey(&a)).To(Equal(EventKey(&b)))

	changed := base()
	changed.Message = "other"
	g.Expect(EventKey(&changed)).ToNot(Equal(EventKey(&a)))

	changed = base()
	changed.Metadata["source.toolkit.fluxcd.io/revision"] = "main@sha1:def"
	g.Expect(EventKey(&changed)).ToNot(Equal(EventKey(&a)))

	changed = base()
	changed.Metadata["source.toolkit.fluxcd.io/token"] = "t1"
	g.Expect(EventKey(&changed)).ToNot(Equal(EventKey(&a)))

	// Metadata of a foreign group does not affect the key.
	changed = base()
	changed.Metadata["kustomize.toolkit.fluxcd.io/revision"] = "main@sha1:def"
	g.Expect(EventKey(&changed)).To(Equal(EventKey(&a)))
}
