package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewMatrix(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		g := NewWithT(t)
		matrix, err := NewMatrix("https://matrix.example.com", "token", "!room:example.com", nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(matrix.URL).To(Equal("https://matrix.example.com"))
		g.Expect(matrix.Token).To(Equal("token"))
		g.Expect(matrix.RoomId).To(Equal("!room:example.com"))
	})

	t.Run("invalid URL", func(t *testing.T) {
		g := NewWithT(t)
		_, err := NewMatrix("not a url", "token", "!room:example.com", nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid Matrix homeserver URL"))
	})
}

func TestMatrix_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.Method).To(Equal(http.MethodPut))
		g.Expect(r.Header.Get("Authorization")).To(HavePrefix("Bearer "))

		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload MatrixPayload
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.MsgType).To(Equal("m.text"))
		g.Expect(payload.Body).To(ContainSubstring("gitrepository/webapp.gitops-system"))
		g.Expect(payload.Body).To(ContainSubstring("message"))
	}))
	defer ts.Close()

	matrix, err := NewMatrix(ts.URL, "test-token", "!test:example.com", nil)
	g.Expect(err).ToNot(HaveOccurred())

	err = matrix.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestMatrix_PostErrorSeverity(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload MatrixPayload
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.Body).To(HavePrefix("🚨"))
	}))
	defer ts.Close()

	matrix, err := NewMatrix(ts.URL, "test-token", "!test:example.com", nil)
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Severity = eventv1.EventSeverityError
	err = matrix.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestMatrix_PostUsesTransactionID(t *testing.T) {
	g := NewWithT(t)
	var requestPaths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
	}))
	defer ts.Close()

	matrix, err := NewMatrix(ts.URL, "test-token", "!test:example.com", nil)
	g.Expect(err).ToNot(HaveOccurred())

	err = matrix.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(requestPaths).To(HaveLen(1))
	g.Expect(requestPaths[0]).To(ContainSubstring("/_matrix/client/r0/rooms/!test:example.com/send/m.room.message/"))
}

func TestMatrix_PostServerError(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	matrix, err := NewMatrix(ts.URL, "test-token", "!test:example.com", nil)
	g.Expect(err).ToNot(HaveOccurred())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = matrix.Post(ctx, testEvent())
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("postMessage failed"))
}

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
