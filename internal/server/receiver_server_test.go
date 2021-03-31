/*
Copyright 2021 The Flux authors

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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
)

func TestReceiverKeyFunc(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup middleware
	store, err := memorystore.New(&memorystore.Config{
		Interval: 10 * time.Minute,
	})
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	middleware, err := httplimit.NewMiddleware(store, receiverKeyFunc)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	handler := middleware.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make request
	tests := []struct {
		digest    string
		rateLimit bool
	}{
		{
			digest:    "1",
			rateLimit: false,
		},
		{
			digest:    "1",
			rateLimit: true,
		},
		{
			digest:    "2",
			rateLimit: false,
		},
		{
			digest:    "3",
			rateLimit: false,
		},
		{
			digest:    "2",
			rateLimit: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.digest, func(t *testing.T) {
			req := httptest.NewRequest("GET", fmt.Sprintf("/hook/%s", tt.digest), nil)
			g.Expect(err).ShouldNot(gomega.HaveOccurred())
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			if tt.rateLimit {
				g.Expect(res.Code).Should(gomega.Equal(429))
				g.Expect(res.Header().Get("X-Ratelimit-Remaining")).Should(gomega.Equal("0"))
			} else {
				g.Expect(res.Code).Should(gomega.Equal(200))
			}
		})
	}
}
