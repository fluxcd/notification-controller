package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	. "github.com/onsi/gomega"
)

func TestDataDogPost(t *testing.T) {
	thisRun := func(expectedToFail bool) func(t *testing.T) {
		return func(t *testing.T) {
			g := NewWithT(t)
			ddApiKey := "sdfsdf"
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
				b, err := io.ReadAll(r.Body)
				g.Expect(err).ToNot(HaveOccurred())
				var payload datadogV1.EventCreateRequest
				err = json.Unmarshal(b, &payload)
				g.Expect(err).ToNot(HaveOccurred())
				if expectedToFail {
					w.WriteHeader(http.StatusForbidden)
				}
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()

			dd, err := NewDataDog(ts.URL, "", nil, ddApiKey)
			g.Expect(err).ToNot(HaveOccurred())

			err = dd.Post(context.Background(), testEvent())
			if expectedToFail {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		}
	}
	t.Run("working", thisRun(false))
	t.Run("failing", thisRun(true))
}

func TestDataDogProviderErrors(t *testing.T) {
	g := NewWithT(t)
	_, err := NewDataDog("https://api.datadoghq.com", "", nil, "")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("token cannot be empty"))

	_, err = NewDataDog("https://bad url :)", "", nil, "token")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to parse address"))
}

func TestToDataDogTags(t *testing.T) {
	g := NewWithT(t)
	dd, err := NewDataDog("https://api.datadoghq.com", "", nil, "token")
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()

	tags := dd.toDataDogTags(&event)

	g.Expect(tags).To(ContainElement("test:metadata"))
	g.Expect(tags).To(ContainElement(fmt.Sprintf("kube_kind:%s", strings.ToLower(event.InvolvedObject.Kind))))
	g.Expect(tags).To(ContainElement(fmt.Sprintf("kube_namespace:%s", event.InvolvedObject.Namespace)))
	g.Expect(tags).To(ContainElement(fmt.Sprintf("kube_name:%s", strings.ToLower(event.InvolvedObject.Name))))
	g.Expect(tags).To(ContainElement(fmt.Sprintf("flux_reporting_controller:%s", strings.ToLower(event.ReportingController))))
	g.Expect(tags).To(ContainElement(fmt.Sprintf("flux_reason:%s", strings.ToLower(event.Reason))))

}
