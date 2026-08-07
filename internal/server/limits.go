/*
Copyright 2026 The Flux authors

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

package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
)

const (
	// maxRequestSizeBytes is the maximum size of a request body accepted by
	// the webhook receiver and the event server, capped to 3 MiB.
	maxRequestSizeBytes = 3 * 1024 * 1024

	// maxHeaderBytes is the maximum size of the request headers accepted by
	// the webhook receiver and the event server, capped to 256 KiB.
	maxHeaderBytes = 256 * 1024

	// readHeaderTimeout is the maximum duration allowed for reading the
	// request headers, capped to 10 seconds.
	readHeaderTimeout = 10 * time.Second

	// readTimeout is the maximum duration allowed for reading the entire
	// request, including the body, capped to 30 seconds.
	readTimeout = 30 * time.Second
)

// readRequestBodyWithLimit reads the request body up to maxRequestSizeBytes
// and replaces r.Body with an in-memory reader over the bytes read, so that
// handlers can read the body again. On failure it logs the error and
// writes the 413 HTTP status code when the body exceeds the maximum
// allowed size, 400 when reading the body fails.
func readRequestBodyWithLimit(logger logr.Logger, w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestSizeBytes+1))
	if err != nil {
		logger.Error(err, "reading the request body failed")
		w.WriteHeader(http.StatusBadRequest)
		return nil, false
	}
	if len(body) > maxRequestSizeBytes {
		logger.Error(fmt.Errorf("request body exceeds the maximum size of %d bytes", maxRequestSizeBytes),
			"reading the request body failed")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return nil, false
	}
	if err := r.Body.Close(); err != nil {
		logger.Error(err, "closing the request body failed")
		w.WriteHeader(http.StatusBadRequest)
		return nil, false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, true
}
