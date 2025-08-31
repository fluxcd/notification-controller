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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestWebex_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload = WebexPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())
	}))
	defer ts.Close()

	webex, err := NewWebex(ts.URL, "", nil, "room", "token")
	g.Expect(err).ToNot(HaveOccurred())

	err = webex.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestWebex_PostUpdate(t *testing.T) {
	g := NewWithT(t)
	webex, err := NewWebex("http://localhost", "", nil, "room", "token")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Metadata[eventv1.MetaCommitStatusKey] = eventv1.MetaCommitStatusUpdateValue
	err = webex.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}
