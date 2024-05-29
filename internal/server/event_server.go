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
	kuberecorder "k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=alerts,verbs=get;list
// +kubebuilder:rbac:groups=notification.toolkit.fluxcd.io,resources=providers,verbs=get

type eventContextKey struct{}

// EventServer handles event POST requests
type EventServer struct {
	port                 string
	logger               logr.Logger
	kubeClient           client.Client
	noCrossNamespaceRefs bool
	detailedMetrics      bool
	kuberecorder.EventRecorder
}

// NewEventServer returns an HTTP server that handles events
func NewEventServer(port string, logger logr.Logger, kubeClient client.Client, eventRecorder kuberecorder.EventRecorder, noCrossNamespaceRefs bool, detailedMetrics bool) *EventServer {
	return &EventServer{
		port:                 port,
		logger:               logger.WithName("event-server"),
		kubeClient:           kubeClient,
		EventRecorder:        eventRecorder,
		noCrossNamespaceRefs: noCrossNamespaceRefs,
		detailedMetrics:      detailedMetrics,
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
		logRateLimitMiddleware,
		s.eventMiddleware,
	} {
		handler = middleware(handler)
	}
	mux := http.NewServeMux()
	path := "/"
	mux.Handle(path, handler)
	middlewarePath := ""
	if !s.detailedMetrics {
		middlewarePath = path
	}
	h := std.Handler(middlewarePath, mdlw, mux)
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

// eventMiddleware cleans up the event metadata using cleanupMetadata() and
// adds the cleaned event in the request context which can then be queried and
// used directly by the other http handlers. This middleware also adds a
// logger with the event's involved object's reference information to the
// request context.
func (s *EventServer) eventMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := r.Body.Close(); err != nil {
			s.logger.Error(err, "closing the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		event := &eventv1.Event{}
		err = json.Unmarshal(body, event)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		cleanupMetadata(event)

		eventLogger := s.logger.WithValues("eventInvolvedObject", event.InvolvedObject)

		enhancedCtx := context.WithValue(r.Context(), eventContextKey{}, event)
		enhancedCtx = log.IntoContext(enhancedCtx, eventLogger)
		enhancedReq := r.WithContext(enhancedCtx)

		h.ServeHTTP(w, enhancedReq)
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

func inList(l []string, i string) bool {
	for _, v := range l {
		if strings.EqualFold(v, i) {
			return true
		}
	}
	return false
}

type statusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

func logRateLimitMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{
			ResponseWriter: w,
			Status:         200,
		}
		h.ServeHTTP(recorder, r)

		if recorder.Status == http.StatusTooManyRequests {
			log.FromContext(r.Context()).V(1).
				Info("Discarding event, rate limiting duplicate events")
		}
	})
}

// eventKeyFunc generates a unique key for an event based on the provided HTTP
// request, which can be used to deduplicate events. The key is calculated by
// concatenating specific event attributes and hashing them using SHA-256.
// The key is then returned as a hex-encoded string.
//
// The event attributes are prefixed with an identifier to avoid collisions
// between different event attributes.
func eventKeyFunc(r *http.Request) (string, error) {
	event := r.Context().Value(eventContextKey{}).(*eventv1.Event)

	comps := []string{
		"event",
		"name=" + event.InvolvedObject.Name,
		"namespace=" + event.InvolvedObject.Namespace,
		"kind=" + event.InvolvedObject.Kind,
		"message=" + event.Message,
	}

	revision, ok := event.Metadata[eventv1.MetaRevisionKey]
	if ok {
		comps = append(comps, "revision="+revision)
	}

	token, ok := event.Metadata[eventv1.MetaTokenKey]
	if ok {
		comps = append(comps, "token="+token)
	}

	key := strings.Join(comps, "/")
	digest := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", digest), nil
}
