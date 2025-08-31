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

func TestGrafana_Post(t *testing.T) {
	t.Run("Successfully post and expect 200 ok", func(t *testing.T) {
		g := NewWithT(t)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			g.Expect(err).ToNot(HaveOccurred())
			var payload = GraphitePayload{}
			err = json.Unmarshal(b, &payload)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(payload.Text).To(Equal("gitrepository/webapp.gitops-system"))
			g.Expect(payload.Tags[0]).To(Equal("flux"))
			g.Expect(payload.Tags[1]).To(Equal("source-controller"))
			g.Expect(payload.Tags[2]).To(Equal("test: metadata"))
			g.Expect(payload.Tags[3]).To(Equal("kind: GitRepository"))
			g.Expect(payload.Tags[4]).To(Equal("name: webapp"))
			g.Expect(payload.Tags[5]).To(Equal("namespace: gitops-system"))
		}))
		defer ts.Close()

		grafana, err := NewGrafana(ts.URL, "", "", nil, "", "")
		g.Expect(err).ToNot(HaveOccurred())

		err = grafana.Post(context.TODO(), testEvent())
		g.Expect(err).ToNot(HaveOccurred())
	})
}
