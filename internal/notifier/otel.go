package notifier

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type OTLPNotifier struct {
	URL       string
	ProxyURL  string
	Headers   map[string]string
	TLSConfig *tls.Config
}

func NewOTELTraceNotifier(url string, proxyURL string, headers map[string]string, tlsConfig *tls.Config) (*OTLPNotifier, error) {
	return &OTLPNotifier{
		URL:       url,
		ProxyURL:  proxyURL,
		Headers:   headers,
		TLSConfig: tlsConfig,
	}, nil
}

// Post implements the notifier.Interface
func (t *OTLPNotifier) Post(ctx context.Context, event eventv1.Event) error {
	logger := log.FromContext(ctx).WithValues(
		"event", event.Reason,
		"object", fmt.Sprintf("%s/%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name),
		"severity", event.Severity,
	)

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
	revision := ""
	for k, v := range event.Metadata {
		if strings.Contains(k, "revision") {
			revision = v
			logger.V(1).Info("Found revision in metadata", "revision", revision)
			break
		}
	}

	// Get value from context (this would need to be passed in from event_handlers.go)
	alertUID, ok := ctx.Value("alertUID").(string)
	if !ok {
		alertUID = "unknown"
		logger.V(1).Info("alertUID not found in context, using default", "alertUID", alertUID)
	} else {
		logger.V(1).Info("Using alertUID from context", "alertUID", alertUID)
	}
	alertName, ok := ctx.Value("alertName").(string)
	if !ok {
		alertUID = "unknown"
		logger.V(1).Info("alertName not found in context, using default", "alertName", alertName)
	} else {
		logger.V(1).Info("Using alertName from context", "alertName", alertName)
	}
	alertNamespace, ok := ctx.Value("alertNamespace").(string)
	if !ok {
		alertNamespace = "unknown"
		logger.V(1).Info("alertNamespace not found in context, using default", "alertNamespace", alertNamespace)
	} else {
		logger.V(1).Info("Using alertNamespace from context", "alertNamespace", alertNamespace)
	}

	// Create trace provider with resource attributes
	logger.V(1).Info("Creating trace provider")
	serviceName := fmt.Sprintf("%s:%s/%s", apiv1beta3.AlertKind, alertNamespace, alertName)
	resource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceInstanceID(alertUID),
		semconv.ServiceName(serviceName),
		semconv.ServiceNamespace(alertNamespace),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
	)

	// Use the trace provider's tracer for span creation
	tracer := tp.Tracer("flux:notification-controller")

	// alertName, ok := ctx.Value("alert.Name").(string)
	// if !ok {
	// 	alertName = "unknown"
	// 	logger.V(1).Info("Alert UID not found in context, using default", "alertUID", alertUID)
	// } else {
	// 	logger.V(1).Info("Using alert UID from context", "alertUID", alertUID)
	// }

	// alertNamespace, ok := ctx.Value("alert.Namespace").(string)
	// if !ok {
	// 	alertNamespace = "unknown"
	// 	logger.V(1).Info("Alert UID not found in context, using default", "alertUID", alertUID)
	// } else {
	// 	logger.V(1).Info("Using alert UID from context", "alertUID", alertUID)
	// }

	// Generate root span ID
	logger.V(1).Info("Generating trace IDs", "alertUID", alertUID, "revision", revision)
	spanIDStr := generateID(string(event.InvolvedObject.UID), revision)
	traceIDStr := generateID(alertUID, revision)

	var traceID trace.TraceID
	var spanID trace.SpanID
	copy(traceID[:], traceIDStr[:16])
	copy(spanID[:], spanIDStr[:8])

	// Create trace context with the generated ID
	var spanCtx context.Context = ctx

	// Replace trace context to use Alert UID + revision
	logger.Info("Trace context", "kind", event.InvolvedObject.Kind)
	// Create new context for root span
	currentSpanContext := trace.SpanContextFromContext(ctx)
	if !currentSpanContext.IsValid() || (currentSpanContext.HasTraceID() &&
		currentSpanContext.TraceID() == traceID) {
		spanCtx = trace.ContextWithSpanContext(ctx,
			trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: traceID,
				// SpanID:     spanID,
				TraceFlags: trace.FlagsSampled, // Ensure the trace is sampled
			}),
		)
	} else {
		logger.V(1).Info("The current Trace is valid and already exists")
	}

	// Create single span with proper attributes
	spanName := fmt.Sprintf("%s:%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
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
		"object", fmt.Sprintf("%s/%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name),
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
