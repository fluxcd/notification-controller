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
	"strings"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

const apiLocations = `{"count":0,"value":[{"area":"","id":"428dd4fb-fda5-4722-af02-9313b80305da","routeTemplate":"","resourceName":"","maxVersion":"6.0","minVersion":"5.0","releasedVersion":"6.0"}]}`

func Fuzz_AzureDevOps(f *testing.F) {
	f.Add("kustomization/gitops-system/0c9c2e41", "alakazam", "org/proj/_git/repo", "revision/dsa123a", "error", "", []byte{}, []byte(`{"count":1,"value":[{"state":"error","description":"","context":{"genre":"fluxcd","name":"/"}}]}`))
	f.Add("kustomization/gitops-system/0c9c2e41", "alakazam", "org/proj/_git/repo", "revision/dsa123a", "info", "", []byte{}, []byte(`{"count":1,"value":[{"state":"info","description":"","context":{"genre":"fluxcd","name":"/"}}]}`))
	f.Add("kustomization/gitops-system/0c9c2e41", "alakazam", "org/proj/_git/repo", "revision/dsa123a", "info", "", []byte{}, []byte(`{"count":0,"value":[]}`))
	f.Add("kustomization/gitops-system/0c9c2e41", "alakazam", "org/proj/_git/repo", "", "", "Progressing", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T, commitStatus, token, urlSuffix, revision, severity, reason string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "_apis") {
				w.Write([]byte(apiLocations))
			} else {
				w.Write(response)
			}

			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		azureDevOps, err := NewAzureDevOps(commitStatus, fmt.Sprintf("%s/%s", ts.URL, urlSuffix), token, &cert)
		if err != nil {
			return
		}

		event := eventv1.Event{}

		// Try to fuzz the event object, but if it fails (not enough seed),
		// ignore it, as other inputs are also being used in this test.
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil && (revision != "") {
			event.Metadata = map[string]string{
				"revision": revision,
			}
		}
		event.Severity = severity

		_ = azureDevOps.Post(context.TODO(), event)
	})
}
