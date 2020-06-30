package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Event is a report of an event issued by a controller.
type Event struct {
	// The object that this event is about.
	// +required
	InvolvedObject corev1.ObjectReference `json:"involvedObject"`

	// Severity type of this event (info, error)
	// +required
	Severity string `json:"severity"`

	// The time at which this event was recorded.
	// +required
	Timestamp metav1.Time `json:"timestamp"`

	// A human-readable description of this event.
	// Maximum length 39,000 characters
	// +optional
	Message string `json:"message,omitempty"`

	// A machine understandable string that gives the reason
	// for the transition into the object's current status.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Metadata of this event, e.g. apply change set.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// Name of the controller that emitted this event, e.g. `source-controller`.
	// +optional
	ReportingController string `json:"reportingController,omitempty"`
}
