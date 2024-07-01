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
	"context"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

// ReceiverServer handles webhook POST requests
type ReceiverServer struct {
	port                  string
	logger                logr.Logger
	kubeClient            client.Client
	exportHTTPPathMetrics bool
}

// NewReceiverServer returns an HTTP server that handles webhooks
func NewReceiverServer(port string, logger logr.Logger, kubeClient client.Client, exportHTTPPathMetrics bool) *ReceiverServer {
	return &ReceiverServer{
		port:                  port,
		logger:                logger.WithName("receiver-server"),
		kubeClient:            kubeClient,
		exportHTTPPathMetrics: exportHTTPPathMetrics,
	}
}

// ListenAndServe starts the HTTP server on the specified port
func (s *ReceiverServer) ListenAndServe(stopCh <-chan struct{}, mdlw middleware.Middleware) {
	mux := http.NewServeMux()
	mux.Handle(apiv1.ReceiverWebhookPath, http.HandlerFunc(s.handlePayload()))
	handlerID := apiv1.ReceiverWebhookPath
	if s.exportHTTPPathMetrics {
		handlerID = ""
	}
	h := std.Handler(handlerID, mdlw, mux)
	srv := &http.Server{
		Addr:    s.port,
		Handler: h,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Error(err, "Receiver server crashed")
			os.Exit(1)
		}
	}()

	// wait for SIGTERM or SIGINT
	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		s.logger.Error(err, "Receiver server graceful shutdown failed")
	} else {
		s.logger.Info("Receiver server stopped")
	}
}
