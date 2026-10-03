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

package v1

// These constants define the event actions emitted by notification-controller.
// The action describes what operation was taken or failed regarding the object,
// and is surfaced on the Flux event/v1 payload sent to notification endpoints.
const (
	// ActionReconciling indicates a reconciliation is in progress.
	ActionReconciling string = "Reconciling"

	// ActionReconciled indicates a successful reconciliation.
	ActionReconciled string = "Reconciled"

	// ActionFetching indicates fetching of a resource or artifact.
	ActionFetching string = "Fetching"

	// ActionValidating indicates validation is in progress.
	ActionValidating string = "Validating"

	// ActionFailed indicates a failed operation.
	ActionFailed string = "Failed"
)
