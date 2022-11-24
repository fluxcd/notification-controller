package notifier

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func Fuzz_Matrix(f *testing.F) {
	f.Add("token", "room1", "", "error", []byte{}, []byte{})
	f.Add("token", "room1", "", "info", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		token, roomId, urlSuffix, severity string, seed, response []byte) {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		matrix, err := NewMatrix(fmt.Sprintf("%s/%s", ts.URL, urlSuffix), token, roomId, &cert)
		if err != nil {
			return
		}

		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		event.Severity = severity

		_ = matrix.Post(context.TODO(), event)
	})
}
