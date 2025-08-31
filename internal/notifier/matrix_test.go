package notifier

import (
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSha1Sum(t *testing.T) {
	g := NewWithT(t)
	timestamp, err := time.Parse("Jan 2, 2006 at 3:04pm (WAT)", "Aug 24, 2021 at 4:18pm (WAT)")
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		event eventv1.Event
		sha1  string
	}{
		{
			event: eventv1.Event{
				InvolvedObject: corev1.ObjectReference{},
				Severity:       eventv1.EventSeverityInfo,
				Timestamp: metav1.Time{
					Time: timestamp,
				},
				Message:             "update successful",
				Reason:              "update sucesful",
				Metadata:            nil,
				ReportingController: "",
				ReportingInstance:   "",
			},
			sha1: "37d91b4f6a1e44c6a38273b0a0fd408fade7b0f5",
		},
	}

	for _, tt := range tests {
		hash, err := sha1sum(tt.event)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(hash).To(Equal(tt.sha1))
	}
}
