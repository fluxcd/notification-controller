//go:build gofuzz
// +build gofuzz

/*
Copyright 2022 The Flux authors

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
	"io"
	"net/http"
	"net/http/httptest"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/fluxcd/pkg/runtime/events"
)

// FuzzGitLab implements a fuzzer that targets GitLab.Post().
func FuzzGitLab(data []byte) int {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		r.Body.Close()
	}))
	defer ts.Close()

	f := fuzz.NewConsumer(data)
	token, err := f.GetString()
	if err != nil {
		return 0
	}

	gitLab, err := NewGitLab(ts.URL, token, nil)
	if err != nil {
		return 0
	}

	event := events.Event{}
	if err := f.GenerateStruct(&event); err != nil {
		return 0
	}

	_ = gitLab.Post(context.TODO(), event)

	return 1
}
