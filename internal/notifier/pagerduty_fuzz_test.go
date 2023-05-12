package notifier

import (
	"context"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func Fuzz_PagerDuty(f *testing.F) {
	f.Add("token", "", "error", "", []byte{}, []byte{})
	f.Add("token", "", "info", "", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		routingKey, commitStatus, severity, message string, seed, response []byte) {
		mux := http.NewServeMux()
		mux.HandleFunc("/v2/enqueue", func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		})
		mux.HandleFunc("/v2/change/enqueue", func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		})
		ts := httptest.NewServer(mux)
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		pd, err := NewPagerDuty(ts.URL, "", &cert, routingKey)
		if err != nil {
			return
		}

		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}

		event.Metadata["commit_status"] = commitStatus
		event.Message = message
		event.Severity = severity

		_ = pd.Post(context.TODO(), event)
	})
}
