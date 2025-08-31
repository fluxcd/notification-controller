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
)

func TestRocket_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload = SlackPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(payload.Attachments[0].AuthorName).To(Equal("gitrepository/webapp.gitops-system"))
		g.Expect(payload.Attachments[0].Fields[0].Value).To(Equal("metadata"))
	}))
	defer ts.Close()

	rocket, err := NewRocket(ts.URL, "", nil, "test", "test")
	g.Expect(err).ToNot(HaveOccurred())

	err = rocket.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestRocket_PostWithoutChannelOrUser(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload = SlackPayload{}
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(payload.Attachments[0].AuthorName).To(Equal("gitrepository/webapp.gitops-system"))
		g.Expect(payload.Attachments[0].Fields[0].Value).To(Equal("metadata"))
	}))
	defer ts.Close()

	rocket, err := NewRocket(ts.URL, "", nil, "", "")
	g.Expect(err).ToNot(HaveOccurred())

	err = rocket.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}
