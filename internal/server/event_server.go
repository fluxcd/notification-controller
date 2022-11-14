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

package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/runtime/events"
)

// EventServer handles event POST requests
type EventServer struct {
	port                 string
	logger               logr.Logger
	kubeClient           client.Client
	noCrossNamespaceRefs bool
	supportHttpScheme    bool
}

// NewEventServer returns an HTTP server that handles events
func NewEventServer(port string, logger logr.Logger, kubeClient client.Client, noCrossNamespaceRefs bool, supportHttpScheme bool) *EventServer {
	return &EventServer{
		port:                 port,
		logger:               logger.WithName("event-server"),
		kubeClient:           kubeClient,
		noCrossNamespaceRefs: noCrossNamespaceRefs,
		supportHttpScheme:    supportHttpScheme,
	}
}

// ListenAndServe starts the HTTP server on the specified port
func (s *EventServer) ListenAndServe(stopCh <-chan struct{}, mdlw middleware.Middleware, store limiter.Store) {
	limitMiddleware, err := httplimit.NewMiddleware(store, eventKeyFunc)
	if err != nil {
		s.logger.Error(err, "Event server crashed")
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.Handle("/", s.logRateLimitMiddleware(limitMiddleware.Handle(http.HandlerFunc(s.handleEvent()))))
	h := std.Handler("", mdlw, mux)
	srv := &http.Server{
		Addr:    s.port,
		Handler: h,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Error(err, "Event server crashed")
			os.Exit(1)
		}
	}()

	// wait for SIGTERM or SIGINT
	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		s.logger.Error(err, "Event server graceful shutdown failed")
	} else {
		s.logger.Info("Event server stopped")
	}
}

type statusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *EventServer) logRateLimitMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{
			ResponseWriter: w,
			Status:         http.StatusOK,
		}
		h.ServeHTTP(recorder, r)

		if recorder.Status == http.StatusTooManyRequests {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				s.logger.Error(err, "reading the request body failed")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			event := &events.Event{}
			err = json.Unmarshal(body, event)
			if err != nil {
				s.logger.Error(err, "decoding the request body failed")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(bytes.NewBuffer(body))

			s.logger.V(1).Info("Discarding event, rate limiting duplicate events",
				"reconciler kind", event.InvolvedObject.Kind,
				"name", event.InvolvedObject.Name,
				"namespace", event.InvolvedObject.Namespace)
		}
	})
}

func eventKeyFunc(r *http.Request) (string, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	event := &events.Event{}
	err = json.Unmarshal(body, event)
	if err != nil {
		return "", err
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	comps := []string{"event", event.InvolvedObject.Name, event.InvolvedObject.Namespace, event.InvolvedObject.Kind, event.Message}
	revString, ok := event.Metadata["revision"]
	if ok {
		comps = append(comps, revString)
	}
	val := strings.Join(comps, "/")
	digest := sha256.Sum256([]byte(val))
	return fmt.Sprintf("%x", digest), nil
}
