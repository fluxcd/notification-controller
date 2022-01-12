//go:build gofuzz
// +build gofuzz

/*
Copyright 2021 The Flux authors

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
	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/fluxcd/pkg/runtime/events"
)

// FuzzNotifierUtil implements a fuzzer that targets
// notifier.formatNameAndDescription() and notifier.parseGitAddress().
func FuzzNotifierUtil(data []byte) int {
	f := fuzz.NewConsumer(data)
	event := events.Event{}

	if err := f.GenerateStruct(&event); err != nil {
		return 0
	}

	_, _ = formatNameAndDescription(event)
	_, _, _ = parseGitAddress(string(data))

	return 1
}
