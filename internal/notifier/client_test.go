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
	"crypto/tls"
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

	. "github.com/onsi/gomega"
)

func Test_postMessage(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload = make(map[string]string)
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload["status"]).To(Equal("success"))
	}))
	defer ts.Close()
	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"})
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_postMessage_timeout(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := postMessage(ctx, ts.URL, map[string]string{"status": "success"})
	g.Expect(err).To(HaveOccurred())
}

func Test_postSelfSignedCert(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		g.Expect(err).ToNot(HaveOccurred())

		var payload = make(map[string]string)
		err = json.Unmarshal(b, &payload)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(payload["status"]).To(Equal("success"))
	}))
	defer ts.Close()

	cert, err := x509.ParseCertificate(ts.TLS.Certificates[0].Certificate[0])
	g.Expect(err).ToNot(HaveOccurred())
	certpool := x509.NewCertPool()
	certpool.AddCert(cert)
	tlsConfig := &tls.Config{RootCAs: certpool}
	err = postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withTLSConfig(tlsConfig))
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_postMessage_requestModifier(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		g.Expect(r.Header.Get("Authorization")).To(Equal("Bearer token"))
	}))
	defer ts.Close()

	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withRequestModifier(func(req *retryablehttp.Request) {
		req.Header.Set("Authorization", "Bearer token")
	}))
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_postMessage_responseValidator(t *testing.T) {
	g := NewWithT(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default response validator determines success, but the custom validator below will determine failure .
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("error: bad request"))
	}))
	defer ts.Close()

	err := postMessage(context.Background(), ts.URL, map[string]string{"status": "success"})
	g.Expect(err).ToNot(HaveOccurred())

	err = postMessage(context.Background(), ts.URL, map[string]string{"status": "success"}, withResponseValidator(func(_ int, body []byte) error {
		if strings.HasPrefix(string(body), "error:") {
			return errors.New(string(body))
		}
		return nil
	}))
	g.Expect(err).To(MatchError(ContainSubstring("request failed: error: bad request")))
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
