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
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func Fuzz_Util_ParseGitAddress(f *testing.F) {
	f.Add("ssh://git@abc.com")

	f.Fuzz(func(t *testing.T, gitAddress string) {
		_, _, _ = parseGitAddress(gitAddress)
	})
}

func Fuzz_Util_FormatNameAndDescription(f *testing.F) {
	f.Add("aA1-", []byte{})

	f.Fuzz(func(t *testing.T, reason string, seed []byte) {
		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		event.Reason = reason

		_, _ = formatNameAndDescription(event)
	})
}
