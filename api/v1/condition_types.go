/*
Copyright 2023 The Flux authors

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

const NotificationFinalizer = "finalizers.fluxcd.io"

const (
	// InitializedReason represents the fact that a given resource has been initialized.
	InitializedReason string = "Initialized"

	// ValidationFailedReason represents the fact that some part of the spec of a given resource
	// couldn't be validated.
	ValidationFailedReason string = "ValidationFailed"

	// TokenNotFoundReason represents the fact that receiver token can't be found.
	TokenNotFoundReason string = "TokenNotFound"

	// MigrationReason represents the fact that a given resource is being
	// migrated to a static resource.
	MigrationReason string = "Migration"

	// InvalidConfigReason represents the fact that part of a given resource's
	// configuration couldn't be used, e.g. an invalid filter regex.
	InvalidConfigReason string = "InvalidConfig"

	// NotificationDispatchFailedReason represents the fact that dispatching a
	// notification to a provider failed.
	NotificationDispatchFailedReason string = "NotificationDispatchFailed"

	// SourceFetchFailedReason represents the fact that the involved object of
	// an event couldn't be fetched to evaluate an alert's source match labels.
	SourceFetchFailedReason string = "SourceFetchFailed"

	// MetadataAppendFailedReason represents the fact that event metadata
	// couldn't be combined due to conflicting keys across the metadata sources.
	MetadataAppendFailedReason string = "MetadataAppendFailed"
)
