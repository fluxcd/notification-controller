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
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

type AlertMetadataContextKey struct{}

type OTLPTracer struct {
	URL       string
	ProxyURL  string
	Headers   map[string]string
	TLSConfig *tls.Config
}

func NewOTLPTracer(url string, proxyURL string, headers map[string]string, tlsConfig *tls.Config) *OTLPTracer {
	return &OTLPTracer{
		URL:       url,
		ProxyURL:  proxyURL,
		Headers:   headers,
		TLSConfig: tlsConfig,
	}
}

// Post implements the notifier.Interface
func (t *OTLPTracer) Post(ctx context.Context, event eventv1.Event) error {
	logger := log.FromContext(ctx).WithValues(
		"event", event.Reason,
		"object", fmt.Sprintf("%s/%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name),
		"severity", event.Severity,
	)
	alert := ctx.Value(AlertMetadataContextKey{}).(metav1.ObjectMeta)

	// Set up OTLP exporter options
	logger.V(1).Info("Configuring OTLP HTTP options", "url", t.URL)
	// Parse URL to extract host and port
	parsedURL, err := url.Parse(t.URL)
	if err != nil {
		logger.Error(err, "Failed to parse URL", "url", t.URL)
		return fmt.Errorf("failed to parse URL: %w", err)
	}
	httpOptions := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(parsedURL.Host),
	}

	// Add headers if available
	if len(t.Headers) > 0 {
		logger.V(1).Info("Adding headers to OTLP exporter", "headerCount", len(t.Headers))
		httpOptions = append(httpOptions, otlptracehttp.WithHeaders(t.Headers))
	}

	// Add TLS config if available
	if t.TLSConfig != nil {
		logger.V(1).Info("Configuring TLS for OTLP exporter")
		httpOptions = append(httpOptions, otlptracehttp.WithTLSClientConfig(t.TLSConfig))
	} else if parsedURL.Scheme == "http" {
		logger.V(1).Info("Using insecure connection for OTLP exporter")
		httpOptions = append(httpOptions, otlptracehttp.WithInsecure())
	}

	// Add proxy if available
	if t.ProxyURL != "" {
		logger.V(1).Info("Setting up Proxy URL for OTLP exporter", "proxyURL", t.ProxyURL)
		proxyURL, err := url.Parse(t.ProxyURL)
		if err != nil {
			logger.Error(err, "Failed to parse proxy URL", "proxyURL", t.ProxyURL)
		} else {
			httpOptions = append(httpOptions, otlptracehttp.WithProxy(func(*http.Request) (*url.URL, error) {
				return proxyURL, nil
			}))
		}
	}

	// Create exporter
	logger.V(1).Info("Creating OTLP exporter")
	exporter, err := otlptracehttp.New(ctx, httpOptions...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Extract revision from event metadata
	revision := extractRevision(event.Metadata)

	// Create trace provider with resource attributes
	logger.V(1).Info("Creating trace provider")
	serviceName := fmt.Sprintf("%s: %s/%s", apiv1beta3.AlertKind, alert.Namespace, alert.Name)
	resource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceInstanceID(string(alert.UID)),
		semconv.ServiceName(serviceName),
		semconv.ServiceNamespace(alert.Namespace),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
	)

	// Tracer instatiation for span creation
	tracer := tp.Tracer("flux:notification-controller")

	// Generate the following IDs:
	// - SpanID: <AlertUID>:<AlertNamespace/AlertName>
	// - TraceID: <AlertUID>:<revisionID>
	logger.V(1).Info("Generating trace IDs", "alertUID", string(alert.UID), "revision", revision)
	traceIDStr := generateID(string(alert.UID), revision)
	spanIDStr := generateID(string(alert.UID), fmt.Sprintf("%s/%s", alert.Namespace, alert.Name))

	var traceID trace.TraceID
	var spanID trace.SpanID
	copy(traceID[:], traceIDStr[:16])
	copy(spanID[:], spanIDStr[:8])

	// Create trace context with the generated ID
	var spanCtx context.Context = ctx

	// Create new context for root span
	currentSpanContext := trace.SpanContextFromContext(ctx)

	// For source objects: create root span with custom traceID
	if isSource(event.InvolvedObject.Kind) {
		logger.V(1).Info("Create a new trace", "traceID", traceID.String())
		spanCtx = trace.ContextWithSpanContext(context.Background(),
			trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    traceID,
				TraceFlags: trace.FlagsSampled,
			}),
		)
	} else {
		// For non-source objects: use existing trace context (becomes child)
		if !currentSpanContext.IsValid() {
			// Fallback: create context with same traceID but no parent
			logger.V(1).Info("Creating an span with a shared traceID", "traceID", traceID.String())
			spanCtx = trace.ContextWithSpanContext(ctx,
				trace.NewSpanContext(trace.SpanContextConfig{
					TraceID:    traceID,
					TraceFlags: trace.FlagsSampled,
				}),
			)
		}
	}

	// Create single span with proper attributes
	spanName := fmt.Sprintf("%s: %s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
	_, span := tracer.Start(spanCtx, spanName,
		trace.WithAttributes(
			attribute.String("flux.object.uid", string(event.InvolvedObject.UID)),
			attribute.String("flux.object.kind", event.InvolvedObject.Kind),
			attribute.String("flux.object.name", event.InvolvedObject.Name),
			attribute.String("flux.object.namespace", event.InvolvedObject.Namespace),
			attribute.String("flux.event.severity", event.Severity),
			attribute.String("flux.event.reason", event.Reason),
			attribute.String("flux.event.message", event.Message),
		),
		trace.WithTimestamp(event.Timestamp.Time),
	)

	// Add metadata attributes
	for k, v := range event.Metadata {
		span.SetAttributes(attribute.String(fmt.Sprintf("flux.event.metadata.%s", k), v))
	}

	// Set status based on event severity
	if event.Severity == eventv1.EventSeverityError {
		span.SetStatus(codes.Error, event.Message)
	} else {
		span.SetStatus(codes.Ok, event.Message)
	}

	logger.Info("Successfully sent trace to OTLP endpoint",
		"url", t.URL,
		"reason", event.Reason)

	defer func() {
		span.End()
		tp.ForceFlush(ctx)
		tp.Shutdown(ctx)
		exporter.Shutdown(ctx)
	}()

	return nil
}

// Add this function to generate trace and span ID
func generateID(alertUID, sourceRevision string) []byte {
	input := fmt.Sprintf("%s:%s", alertUID, sourceRevision)
	hash := sha256.Sum256([]byte(input))
	return hash[:]
}

func extractRevision(metadata map[string]string) string {
	for k, v := range metadata {
		if strings.Contains(k, "revision") {
			return v
		}
	}
	return "unknown"
}

func isSource(kind string) bool {
	sourceKinds := []string{"GitRepository", "HelmRepository", "OCIRepository", "Bucket"}
	return slices.Contains(sourceKinds, kind)
}
