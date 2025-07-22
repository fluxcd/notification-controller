package notifier

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/stretchr/testify/require"
)

func Fuzz_DataDog(f *testing.F) {
	f.Add("token", "error", "", []byte{}, []byte{})
	f.Add("token", "info", "", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		apiKey, severity, message string, seed, response []byte) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write(response)
			require.NoError(t, err)
			_, err = io.Copy(io.Discard, r.Body)
			require.NoError(t, err)
			require.NoError(t, r.Body.Close())
		})
		ts := httptest.NewServer(mux)
		defer ts.Close()

		var cert x509.CertPool
		_ = fuzz.NewConsumer(seed).GenerateStruct(&cert)

		tlsConfig := &tls.Config{RootCAs: &cert}
		dd, err := NewDataDog(ts.URL, "", tlsConfig, apiKey)
		if err != nil {
			return
		}

		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}

		event.Message = message
		event.Severity = severity

		_ = dd.Post(context.TODO(), event)
	})
}
