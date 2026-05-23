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
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
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
	kubeClient     client.Client
}

// IDGenerator generates deterministic IDs for Flux objects
type IDGenerator struct {
	UID      string
	revision string
}

// Context keys for IDGenerator
type ObjUID struct{}
type ObjRevision struct{}

func (g *IDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	objectUID, _ := ctx.Value(ObjUID{}).(string)
	objectRevision, _ := ctx.Value(ObjRevision{}).(string)

	var traceID trace.TraceID
	var spanID trace.SpanID

	// TraceID from alert (UID + Root Source)
	traceIDBytes := generateID(g.UID, g.revision)
	copy(traceID[:], traceIDBytes[:16])

	// SpanID from object
	spanIDBytes := generateID(objectUID, objectRevision)
	copy(spanID[:], spanIDBytes[:8])

	return traceID, spanID
}

func (g *IDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	_, spanID := g.NewIDs(ctx)
	return spanID
}

func NewOTLPTracer(ctx context.Context, kubeClient client.Client, urlStr string, proxyURL string, headers map[string]string, tlsConfig *tls.Config, username string, password string) (*OTLPTracer, error) {
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
		kubeClient:     kubeClient,
	}, nil
}

// Post implements the notifier.Interface
func (t *OTLPTracer) Post(ctx context.Context, event eventv1.Event) error {
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

	kind := event.InvolvedObject.Kind

	// Skip if it's HelmRepository kind object (no considered as main source for tracing)
	if kind == "HelmRepository" {
		logger.Info("OTEL notification skipped", "alert", alert.Namespace, alert.Name)
		return nil
	}

	// Check if the object is part of a Kustomization
	generators := t.getKustFromLabels(ctx, event.InvolvedObject)
	parentRevision := getRevision(event)

	var parentUID string = ""
	var revision string = ""

	revision = parentRevision
	if len(generators) > 0 {
		// Object is under a Kustomization
		root := generators[len(generators)-1]
		revision = root.revision

		if isSource(kind) {
			// Source: parent is immediate Kustomization
			parentUID = generators[0].UID
			parentRevision = generators[0].revision
		} else if parentRevision == revision {
			// Non-source: parent is root Kustomization
			parentUID = root.UID
		} else {
			// Non-source with different revision: use object's direct source
			parentUID = t.getParentUID(ctx, event)
		}
	} else if !isSource(kind) {
		// No Kustomization parent
		parentUID = t.getParentUID(ctx, event)
	}

	logger.Info("Generating trace IDs", "alertUID", string(alert.UID), "revision", revision)

	// Create TraceProvider
	provider := t.createProvider(alert, revision)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			logger.Error(err, "Failed to shutdown tracer provider")
		}
	}()

	// Determine span relationship based on Flux object hierarchy
	spanCtx := createSpanContext(ctx, event,
		&IDGenerator{UID: string(alert.UID), revision: revision}, // Root
		&IDGenerator{UID: parentUID, revision: parentRevision},   // Parent
	)

	// Create single span with proper attributes
	ctx, span := processSpan(provider, spanCtx, event)

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

func (t *OTLPTracer) createProvider(alert metav1.ObjectMeta, revision string) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(t.tracerExporter),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceName(fmt.Sprintf("%s/%s", alert.GetNamespace(), alert.GetName())),
				semconv.ServiceNamespace(alert.GetNamespace()),
				semconv.ServiceInstanceID(string(alert.GetUID())),
			),
		),
		sdktrace.WithIDGenerator(&IDGenerator{
			UID:      string(alert.GetUID()),
			revision: revision,
		}),
	)
}

func (t *OTLPTracer) getParentUID(ctx context.Context, event eventv1.Event) string {
	obj, err := t.getObject(ctx, event.InvolvedObject)
	if err != nil {
		return ""
	}

	kind := event.InvolvedObject.Kind
	var name string = ""
	var namespace string = ""

	// HelmRelease: check chartRef for HelmChart parent
	if chartRef, found, _ := unstructured.NestedMap(obj.Object, "spec", "chartRef"); found {
		kind, _ = chartRef["kind"].(string)
		name, _ = chartRef["name"].(string)
		namespace, _ = chartRef["namespace"].(string)
		if namespace == "" {
			namespace = event.InvolvedObject.Namespace
		}
	}

	// Check sourceRef (for GitRepo>Kust, HelmRepo>HelmChart, OCIRepo>HelmRelease, etc)
	if sourceRef, found, _ := unstructured.NestedMap(obj.Object, "spec", "sourceRef"); found {
		kind, _ = sourceRef["kind"].(string)
		if kind == "HelmRepository" {
			kind = "HelmChart"
		}
		name, _ = sourceRef["name"].(string)
		namespace, _ = sourceRef["namespace"].(string)
		if namespace == "" {
			namespace = event.InvolvedObject.Namespace
		}
	}

	if name != "" {
		if meta, err := t.getObjectPartialMetadata(ctx, corev1.ObjectReference{
			APIVersion: "source.toolkit.fluxcd.io/v1",
			Kind:       kind,
			Name:       name,
			Namespace:  namespace,
		}); err == nil {
			return string(meta.GetUID())
		}
	}

	return ""
}

func (t *OTLPTracer) getKustFromLabels(ctx context.Context, objRef corev1.ObjectReference) []*IDGenerator {
	logger := log.FromContext(ctx).V(1)
	var generators []*IDGenerator

	ksName, ksNamespace := t.retrieveKustLabels(ctx, objRef)
	if ksName == "" || ksNamespace == "" {
		return generators
	}

	kustRef := corev1.ObjectReference{
		APIVersion: kustomizev1.GroupVersion.String(),
		Kind:       kustomizev1.KustomizationKind,
		Name:       ksName,
		Namespace:  ksNamespace,
	}

	kust, err := t.getObject(ctx, kustRef)
	if err != nil {
		logger.Info("Failed to get Kustomization", "error", err)
		return generators
	}

	revision := extractRevision(kust)
	uid := string(kust.GetUID())

	// Add current Kustomization to generators list
	generators = append(generators, &IDGenerator{
		UID:      uid,
		revision: revision,
	})

	// Recursive call: check if this Kustomization is managed by another one
	parentGenerators := t.getKustFromLabels(ctx, kustRef)
	if len(parentGenerators) > 0 {
		generators = append(generators, parentGenerators...)
	}

	// This is the root Kustomization
	logger.Info("Got Parent Kustomization", "uid", uid, "revision", revision)

	return generators
}

func (t *OTLPTracer) retrieveKustLabels(ctx context.Context, objRef corev1.ObjectReference) (string, string) {
	logger := log.FromContext(ctx).V(1)
	objMeta, err := t.getObjectPartialMetadata(ctx, objRef)
	if err != nil {
		logger.Info("Failed to get object metadata", "error", err)
		return "", ""
	}

	labels := objMeta.GetLabels()
	ksName := labels["kustomize.toolkit.fluxcd.io/name"]
	ksNamespace := labels["kustomize.toolkit.fluxcd.io/namespace"]

	if ksName == "" || ksNamespace == "" {
		logger.Info("No Kustomization labels found under", "objRefName", objRef.Name, "objRefNamespace", objRef.Namespace)
		return ksName, ksNamespace
	}

	logger.Info("Found Kustomization labels", "ksName", ksName, "ksNamespace", ksNamespace)

	return ksName, ksNamespace
}

func (t *OTLPTracer) getObject(ctx context.Context, objRef corev1.ObjectReference) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.FromAPIVersionAndKind(objRef.APIVersion, objRef.Kind))

	if err := t.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: objRef.Namespace,
		Name:      objRef.Name,
	}, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

func (t *OTLPTracer) getObjectPartialMetadata(ctx context.Context, objRef corev1.ObjectReference) (*metav1.PartialObjectMetadata, error) {
	meta := &metav1.PartialObjectMetadata{}
	meta.SetGroupVersionKind(schema.FromAPIVersionAndKind(objRef.APIVersion, objRef.Kind))

	if err := t.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: objRef.Namespace,
		Name:      objRef.Name,
	}, meta); err != nil {
		return nil, err
	}

	return meta, nil
}

func extractRevision(obj *unstructured.Unstructured) string {
	if rev, found, _ := unstructured.NestedString(obj.Object, "status", "lastAppliedRevision"); found {
		return rev
	}
	if rev, found, _ := unstructured.NestedString(obj.Object, "status", "artifact", "revision"); found {
		return rev
	}
	return "unknown"
}

func createSpanContext(ctx context.Context, event eventv1.Event, rootSpan *IDGenerator, parentSpan *IDGenerator) context.Context {

	// No parent and non-source object
	if parentSpan.UID != "" {
		var traceID trace.TraceID
		var spanID trace.SpanID

		parentSpanID := generateID(parentSpan.UID, parentSpan.revision)
		traceIDStr := generateID(rootSpan.UID, rootSpan.revision)
		copy(traceID[:], traceIDStr[:16])
		copy(spanID[:], parentSpanID[:8])

		parentSpanCtx := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
		})
		ctx = trace.ContextWithSpanContext(ctx, parentSpanCtx)
	}

	// Set current object's UID and revision for span generation
	ctx = context.WithValue(ctx, ObjUID{}, string(event.InvolvedObject.UID))
	ctx = context.WithValue(ctx, ObjRevision{}, getRevision(event))

	return ctx
}

func processSpan(tracerProvider *sdktrace.TracerProvider, ctx context.Context, event eventv1.Event) (context.Context, trace.Span) {
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
	return tracer.Start(ctx, spanName,
		trace.WithAttributes(eventAttrs...),
		trace.WithTimestamp(event.Timestamp.Time),
	)
}

// Build the revision ID based on the event metadata
func getRevision(event eventv1.Event) string {
	revision, hasRev := event.GetRevision()
	if !hasRev {
		return "unknown"
	}

	// OCIRepositories does populate the following metadata
	// which it's the same revision as some other sources
	// <app-version>@<oci-digest> -> <version>@<algorithm>:<checksum>
	ociDigest, hasOCI := event.Metadata["oci-digest"]
	appVersion, hasApp := event.Metadata["app-version"]

	if rev, hasRev := event.Metadata["source-revision"]; hasRev {
		revision = rev
	} else if hasOCI && hasApp {
		revision = appVersion + "@" + ociDigest
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
	sourceKinds := []string{"GitRepository", "HelmChart", "OCIRepository", "Bucket", "ExternalArtifact"}
	return slices.Contains(sourceKinds, kind)
}
