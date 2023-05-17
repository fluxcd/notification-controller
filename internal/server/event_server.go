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

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type eventContextKey struct{}

// EventServer handles event POST requests
type EventServer struct {
	port                 string
	logger               logr.Logger
	kubeClient           client.Client
	noCrossNamespaceRefs bool
}

// NewEventServer returns an HTTP server that handles events
func NewEventServer(port string, logger logr.Logger, kubeClient client.Client, noCrossNamespaceRefs bool) *EventServer {
	return &EventServer{
		port:                 port,
		logger:               logger.WithName("event-server"),
		kubeClient:           kubeClient,
		noCrossNamespaceRefs: noCrossNamespaceRefs,
	}
}

// ListenAndServe starts the HTTP server on the specified port
func (s *EventServer) ListenAndServe(stopCh <-chan struct{}, mdlw middleware.Middleware, store limiter.Store) {
	limitMiddleware, err := httplimit.NewMiddleware(store, eventKeyFunc)
	if err != nil {
		s.logger.Error(err, "Event server crashed")
		os.Exit(1)
	}
	var handler http.Handler = http.HandlerFunc(s.handleEvent())
	for _, middleware := range []func(http.Handler) http.Handler{
		limitMiddleware.Handle,
		s.logRateLimitMiddleware,
		s.cleanupMetadataMiddleware,
	} {
		handler = middleware(handler)
	}
	mux := http.NewServeMux()
	mux.Handle("/", handler)
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

// cleanupMetadataMiddleware cleans up the metadata using cleanupMetadata() and
// adds the cleaned event in the request context which can then be queried and
// used directly by the other http handlers.
func (s *EventServer) cleanupMetadataMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		event := &eventv1.Event{}
		err = json.Unmarshal(body, event)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		cleanupMetadata(event)

		ctxWithEvent := context.WithValue(r.Context(), eventContextKey{}, event)
		reqWithEvent := r.WithContext(ctxWithEvent)

		h.ServeHTTP(w, reqWithEvent)
	})
}

// cleanupMetadata removes metadata entries which are not used for alerting.
func cleanupMetadata(event *eventv1.Event) {
	group := event.InvolvedObject.GetObjectKind().GroupVersionKind().Group
	excludeList := []string{
		fmt.Sprintf("%s/%s", group, eventv1.MetaChecksumKey),
		fmt.Sprintf("%s/%s", group, eventv1.MetaDigestKey),
	}

	meta := make(map[string]string)
	if event.Metadata != nil && len(event.Metadata) > 0 {
		// Filter other meta based on group prefix, while filtering out excludes
		for key, val := range event.Metadata {
			if strings.HasPrefix(key, group) && !inList(excludeList, key) {
				newKey := strings.TrimPrefix(key, fmt.Sprintf("%s/", group))
				meta[newKey] = val
			}
		}
	}

	event.Metadata = meta
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
			Status:         200,
		}
		h.ServeHTTP(recorder, r)

		if recorder.Status == http.StatusTooManyRequests {
			event := r.Context().Value(eventContextKey{}).(*eventv1.Event)
			s.logger.V(1).Info("Discarding event, rate limiting duplicate events",
				"reconciler kind", event.InvolvedObject.Kind,
				"name", event.InvolvedObject.Name,
				"namespace", event.InvolvedObject.Namespace)
		}
	})
}

func eventKeyFunc(r *http.Request) (string, error) {
	event := r.Context().Value(eventContextKey{}).(*eventv1.Event)

	comps := []string{
		"event",
		event.InvolvedObject.Name,
		event.InvolvedObject.Namespace,
		event.InvolvedObject.Kind,
		event.Message,
	}

	revString, ok := event.Metadata[eventv1.MetaRevisionKey]
	if ok {
		comps = append(comps, revString)
	}

	val := strings.Join(comps, "/")
	digest := sha256.Sum256([]byte(val))
	return fmt.Sprintf("%x", digest), nil
}
