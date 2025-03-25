/*
Copyright 2025 The Flux authors

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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testHTTPProxyServer struct {
	*httptest.Server
}

// newTestHTTPProxyServer returns a test HTTP proxy server which can handle both HTTP and HTTPS traffic
func newTestHTTPProxyServer(t *testing.T) testHTTPProxyServer {
	testHTTPProxyServer := testHTTPProxyServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			testHTTPProxyServer.tunnel(t, w, r)
		} else {
			testHTTPProxyServer.forward(t, w, r)
		}
	}))
	testHTTPProxyServer.Server = server

	return testHTTPProxyServer
}

func (s testHTTPProxyServer) forward(t *testing.T, w http.ResponseWriter, r *http.Request) {
	// forward the request to destination
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		t.Logf("error forwarding the request: %v", err)
		return
	}

	// copy the headers from the proxy response to the original response
	for headerName, values := range resp.Header {
		for _, headerValue := range values {
			w.Header().Add(headerName, headerValue)
		}
	}

	// set the status code of the original response to the status code of the proxy response
	w.WriteHeader(resp.StatusCode)

	// copy the body of the proxy response to the original response
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		t.Logf("error copying response body: %v", err)
	}
}

func (s testHTTPProxyServer) tunnel(t *testing.T, w http.ResponseWriter, r *http.Request) {
	dialer := net.Dialer{}
	// establish a TCP connection to destination
	serverConn, err := dialer.DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		t.Logf("failed to establish a TCP connecton to the: %s", r.Host)
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	defer serverConn.Close()

	// try to hijack the TCP connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Log("failed to hijack the TCP connection")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	// take over the TCP connection to the client
	clientConn, bufClientConn, err := hj.Hijack()
	if err != nil {
		t.Logf("failed to take over the TCP connection: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// tunnel the 'client > server' data
	go s.copyProxyData(t, serverConn, bufClientConn)
	// tunnel the 'server > client' data
	s.copyProxyData(t, bufClientConn, serverConn)
}

func (s testHTTPProxyServer) copyProxyData(t *testing.T, dst io.Writer, src io.Reader) {
	_, err := io.Copy(dst, src)
	if err != nil {
		t.Logf("error copying data: %v", err)
	}
}
