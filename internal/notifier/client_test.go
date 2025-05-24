/*
Copyright 2020 The Flux authors

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

package notifier

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/hashicorp/go-retryablehttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

func Test_postMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload = make(map[string]string)
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		require.Equal(t, "success", payload["status"])
	}))
	defer ts.Close()
	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"})
	require.NoError(t, err)
}

func Test_postMessage_timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := postMessage(ctx, ts.URL, map[string]string{"status": "success"})
	require.Error(t, err, "context deadline exceeded")
}

func Test_postSelfSignedCert(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload = make(map[string]string)
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		require.Equal(t, "success", payload["status"])
	}))
	defer ts.Close()

	cert, err := x509.ParseCertificate(ts.TLS.Certificates[0].Certificate[0])
	require.NoError(t, err)
	certpool := x509.NewCertPool()
	certpool.AddCert(cert)
	err = postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withCertPool(certpool))
	require.NoError(t, err)
}

func Test_postMessage_requestModifier(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
	}))
	defer ts.Close()

	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withRequestModifier(func(req *retryablehttp.Request) {
		req.Header.Set("Authorization", "Bearer token")
	}))
	require.NoError(t, err)
}

func Test_postMessage_responseValidator(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default response validator determines success, but the custom validator below will determine failure .
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("error: bad request"))
	}))
	defer ts.Close()

	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"})
	require.NoError(t, err)

	err = postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withResponseValidator(func(_ int, body []byte) error {
		if strings.HasPrefix(string(body), "error:") {
			return errors.New(string(body))
		}
		return nil
	}))
	require.ErrorContains(t, err, "request failed: error: bad request")
}

func testEvent() eventv1.Event {
	return eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "GitRepository",
			Namespace: "gitops-system",
			Name:      "webapp",
		},
		Severity:  "info",
		Timestamp: metav1.Now(),
		Message:   "message",
		Reason:    "reason",
		Metadata: map[string]string{
			"test": "metadata",
		},
		ReportingController: "source-controller",
		ReportingInstance:   "source-controller-xyz",
	}
}
