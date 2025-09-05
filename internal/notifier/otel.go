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
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"

	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
)

// Context key
type alertMetadataContextKey struct{}

func WithAlertMetadata(ctx context.Context, metadata metav1.ObjectMeta) context.Context {
	return context.WithValue(ctx, alertMetadataContextKey{}, metadata)
}

func GetAlertMetadata(ctx context.Context) (metav1.ObjectMeta, bool) {
	metadata, ok := ctx.Value(alertMetadataContextKey{}).(metav1.ObjectMeta)
	return metadata, ok
}

type OTLPTracer struct {
	tracerExporter *otlptrace.Exporter
}

func NewOTLPTracer(ctx context.Context, urlStr string, proxyURL string, headers map[string]string, tlsConfig *tls.Config, username string, password string) (*OTLPTracer, error) {
	// Set up OTLP exporter options
	httpOptions := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(urlStr),
	}

	// Add headers if available
	if len(headers) > 0 {
		// Add authentication header, if it doesn't exist yet
		if headers["Authorization"] == "" {
			// If username is not set, password is considered as token
			if username == "" {
				headers["Authorization"] = "Bearer " + password
			} else if password != "" {
				auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
				headers["Authorization"] = "Basic " + auth
			}
		}
		httpOptions = append(httpOptions, otlptracehttp.WithHeaders(headers))
	}

	// Add TLS config if available
	if tlsConfig != nil {
		httpOptions = append(httpOptions, otlptracehttp.WithTLSClientConfig(tlsConfig))
	}

	// Add proxy if available
	if proxyURL != "" {
		proxyURLparsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("failed to proxy URL - %s: %w", proxyURL, err)
		} else {
			if username != "" && password != "" {
				proxyURLparsed.User = url.UserPassword(username, password)
			}
			httpOptions = append(httpOptions, otlptracehttp.WithProxy(func(*http.Request) (*url.URL, error) {
				return proxyURLparsed, nil
			}))
		}
	}

	exporter, err := otlptracehttp.New(ctx, httpOptions...)
	if err != nil {
		return nil, err
	}

	log.FromContext(ctx).V(1).Info("Successfully created OTEL tracerExporter")
	return &OTLPTracer{
		tracerExporter: exporter,
	}, nil
}

// Post implements the notifier.Interface
func (t *OTLPTracer) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	logger := log.FromContext(ctx).V(1).WithValues(
		"event", event.Reason,
		"object", fmt.Sprintf("%s/%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name),
		"severity", event.Severity,
	)
	logger.Info("OTEL Post function called", "event", event.Reason)

	alert, ok := GetAlertMetadata(ctx)
	if !ok {
		return fmt.Errorf("alert metadata not found in context")
	}

	// Extract revision from event metadata
	revision := getRevision(event.Metadata)

	// Create TraceProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(t.tracerExporter)),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceName(fmt.Sprintf("%s/%s", alert.GetNamespace(), alert.GetName())),
				semconv.ServiceNamespace(alert.GetNamespace()),
				semconv.ServiceInstanceID(string(alert.GetUID())),
			),
		),
	)

	// Generate traceID
	logger.V(1).Info("Generating trace IDs", "alertUID", string(alert.UID), "revision", revision)
	var traceID trace.TraceID
	traceIDStr := generateID(string(alert.UID), revision)
	copy(traceID[:], traceIDStr[:16])

	// Determine span relationship based on Flux object hierarchy
	var spanCtx context.Context = createSpanContext(ctx, event, traceID)

	// Skip if it's HelmRepository kind object (no considered as main source for tracing)
	if event.InvolvedObject.Kind != "HelmRepository" {
		logger.Info("Processing OTEL notification", "alert", alert.Name)

	} else {
		logger.Info("OTEL notification skipped", "alert", alert.Name)
		return nil
	}

	// Create single span with proper attributes
	span := processSpan(tp, spanCtx, event)
	// Set status based on event severity
	if event.Severity == eventv1.EventSeverityError {
		span.SetStatus(codes.Error, event.Message)
	} else {
		span.SetStatus(codes.Ok, event.Message)
	}

	defer span.End()

	serviceName := fmt.Sprintf("%s: %s/%s", apiv1beta3.AlertKind, alert.Namespace, alert.Name)
	logger.Info("Successfully sent trace to OTLP endpoint",
		"alert", serviceName,
	)

	return nil
}

func createSpanContext(ctx context.Context, event eventv1.Event, traceID trace.TraceID) context.Context {
	kind := event.InvolvedObject.Kind

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	// Root spans: Sources that start the deployment flow
	if isSource(kind) {
		return trace.ContextWithSpanContext(context.Background(),
			spanContext.WithTraceFlags(spanContext.TraceFlags()))
	}

	// Child spans: Everything else inherits from the same trace
	return trace.ContextWithSpanContext(ctx,
		spanContext.WithTraceFlags(spanContext.TraceFlags()))
}

func processSpan(tracerProvider *sdktrace.TracerProvider, ctx context.Context, event eventv1.Event) trace.Span {
	// Build span attributes including metadata
	eventAttrs := []attribute.KeyValue{
		attribute.String("object.uid", string(event.InvolvedObject.UID)),
		attribute.String("object.kind", event.InvolvedObject.Kind),
		attribute.String("object.name", event.InvolvedObject.Name),
		attribute.String("object.namespace", event.InvolvedObject.Namespace),
	}

	// Add event metadata as span attributes
	for k, v := range event.Metadata {
		eventAttrs = append(eventAttrs, attribute.String(k, v))
	}

	// Create tracer and start tracing
	spanName := fmt.Sprintf("%s: %s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
	tracer := tracerProvider.Tracer("flux:notification-controller")
	_, span := tracer.Start(ctx, spanName,
		trace.WithAttributes(eventAttrs...),
		trace.WithTimestamp(event.Timestamp.Time))

	return span
}

// Build the revision ID based on the event metadata
func getRevision(eventMetadata map[string]string) string {
	var revision string = "unknown"

	// OCIRepositories does populate the following metadata
	// which it's the same revision as some other sources
	// <app-version>@<oci-digest> -> <version>@<algorithm>:<checksum>
	ociDigest, hasOCI := eventMetadata["oci-digest"]
	appVersion, hasApp := eventMetadata["app-version"]

	if hasOCI && hasApp {
		revision = appVersion + "@" + ociDigest
	} else if rev, hasRev := eventMetadata["revision"]; hasRev {
		revision = rev
	}

	return revision
}

// Generate IDs based on: UID + revision
func generateID(UID string, revision string) []byte {
	input := fmt.Sprintf("%s:%s", UID, revision)
	hash := sha256.Sum256([]byte(input))
	return hash[:]
}

// Discriminates if an object kind is a source
func isSource(kind string) bool {
	sourceKinds := []string{"GitRepository", "HelmChart", "OCIRepository", "Bucket"}
	return slices.Contains(sourceKinds, kind)
}
