package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
)

func TestLark_Post(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())
		var payload LarkPayload
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload.Card.Header.Title.Content).To(Equal("💫 gitrepository/webapp.gitops-system"))
		g.Expect(payload.Card.Header.Template).To(Equal("turquoise"))
	}))
	defer ts.Close()

	lark, err := NewLark(ts.URL)
	g.Expect(err).ToNot(HaveOccurred())

	err = lark.Post(context.TODO(), testEvent())
	g.Expect(err).ToNot(HaveOccurred())
}
