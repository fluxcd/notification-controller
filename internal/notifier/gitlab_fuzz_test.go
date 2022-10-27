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
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func Fuzz_GitLab(f *testing.F) {
	f.Add("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "token", "org/repo", "revision/abce1", "error", "", []byte{}, []byte(`[{"sha":"abce1","status":"failed","name":"/","description":""}]`))
	f.Add("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "token", "org/repo", "revision/abce1", "info", "", []byte{}, []byte(`[{"sha":"abce1","status":"failed","name":"/","description":""}]`))
	f.Add("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "token", "org/repo", "revision/abce1", "info", "Progressing", []byte{}, []byte{})
	f.Add("0c9c2e41-d2f9-4f9b-9c41-bebc1984d67a", "token", "org/repo", "revision/abce1", "info", "", []byte{}, []byte(`[]`))

	f.Fuzz(func(t *testing.T, uuid, token, urlSuffix, revision, severity, reason string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		gitLab, err := NewGitLab(uuid, fmt.Sprintf("%s/%s", ts.URL, urlSuffix), token, &cert)
		if err != nil {
			return
		}

		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil && (revision != "") {
			event.Metadata = map[string]string{
				"revision": revision,
			}
		}

		event.Severity = severity
		event.Reason = reason

		_ = gitLab.Post(context.TODO(), event)
	})
}
