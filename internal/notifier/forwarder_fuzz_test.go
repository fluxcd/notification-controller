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
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func Fuzz_Forwarder(f *testing.F) {
	f.Add("", []byte{}, []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		urlSuffix string, seed, response, hmacKey []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var tlsConfig tls.Config
		_ = fuzz.NewConsumer(seed).GenerateStruct(&tlsConfig)

		header := make(map[string]string)
		_ = fuzz.NewConsumer(seed).FuzzMap(&header)

		forwarder, err := NewForwarder(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), "", header, &tlsConfig, hmacKey)
		if err != nil {
			return
		}

		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		_ = forwarder.Post(context.TODO(), event)
	})
}
