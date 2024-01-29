package scout

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/aws/smithy-go/ptr"
	"github.com/samber/lo"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

const OTLPDefaultEndpoint = "https://otel.getscout.us:4318"

const ErrorURLAttribute = "URL"

const ProjectIDAttribute = "scout.project_id"
const SessionIDAttribute = "scout.session_id"
const RequestIDAttribute = "scout.trace_id"
const SourceAttribute = "scout.source"
const TraceTypeAttribute = "scout.type"
const TraceKeyAttribute = "scout.key"

const LogEvent = "log"
const LogSeverityAttribute = "log.severity"
const LogMessageAttribute = "log.message"

const MetricEvent = "metric"
const MetricEventName = "metric.name"
const MetricEventValue = "metric.value"

type TraceType string

const TraceTypeNetworkRequest TraceType = "http.request"
const TraceTypeScoutInternal TraceType = "scout.internal"

type OTLP struct {
	tracerProvider *sdktrace.TracerProvider
}

type ErrorWithStack interface {
	Error() string
	StackTrace() errors.StackTrace
}

type sampler struct {
	traceIDUpperBounds map[trace.SpanKind]uint64
	description        string
}

func (s sampler) ShouldSample(sp sdktrace.SamplingParameters) sdktrace.SamplingResult {
	psc := trace.SpanContextFromContext(sp.ParentContext)
	x := binary.BigEndian.Uint64(sp.TraceID[8:16]) >> 1
	bound, ok := s.traceIDUpperBounds[sp.Kind]
	if !ok {
		bound = s.traceIDUpperBounds[trace.SpanKindUnspecified]
	}
	if x < bound {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: psc.TraceState(),
		}
	}
	return sdktrace.SamplingResult{
		Decision:   sdktrace.Drop,
		Tracestate: psc.TraceState(),
	}
}

func (s sampler) Description() string {
	return s.description
}

// creates a per-span-kind sampler that samples each kind at a provided fraction.
func getSampler() sampler {
	return sampler{
		description: fmt.Sprintf("TraceIDRatioBased{%+v}", conf.samplingRateMap),
		traceIDUpperBounds: lo.MapEntries(conf.samplingRateMap, func(key trace.SpanKind, value float64) (trace.SpanKind, uint64) {
			return key, uint64(value * (1 << 63))
		}),
	}
}

var (
	tracer = otel.GetTracerProvider().Tracer(
		"github.com/scout-inc/scout-go",
		trace.WithInstrumentationVersion("v0.1.0"),
		trace.WithSchemaURL(semconv.SchemaURL),
	)
)

func StartOTLP() (*OTLP, error) {
	var options []otlptracehttp.Option
	if strings.HasPrefix(conf.otelEndpoint, "http://") {
		options = append(options, otlptracehttp.WithEndpoint(conf.otelEndpoint[7:]), otlptracehttp.WithInsecure())
	} else if strings.HasPrefix(conf.otelEndpoint, "https://") {
		options = append(options, otlptracehttp.WithEndpoint(conf.otelEndpoint[8:]))
	} else {
		logger.Errorf("an invalid otlp endpoint was configured %s", conf.otelEndpoint)
	}
	options = append(options, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
	client := otlptracehttp.NewClient(options...)
	exporter, err := otlptrace.New(context.Background(), client)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}
	otelResource, err := resource.New(context.Background(),
		resource.WithFromEnv(),
		resource.WithHost(),
		resource.WithContainer(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithAttributes(conf.resourceAttributes...),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP resource context: %w", err)
	}
	h := &OTLP{
		tracerProvider: sdktrace.NewTracerProvider(
			sdktrace.WithSampler(getSampler()),
			sdktrace.WithBatcher(
				exporter,
				sdktrace.WithBatchTimeout(1000*time.Millisecond),
				sdktrace.WithMaxExportBatchSize(128),
				sdktrace.WithMaxQueueSize(1024)),
			sdktrace.WithResource(otelResource),
		),
	}
	otel.SetTracerProvider(h.tracerProvider)
	return h, nil
}

func (o *OTLP) shutdown() {
	err := o.tracerProvider.ForceFlush(context.Background())
	if err != nil {
		logger.Error(err)
	}
	err = o.tracerProvider.Shutdown(context.Background())
	if err != nil {
		logger.Error(err)
	}
}

func StartTraceWithTimestamp(ctx context.Context, name string, t time.Time, opts []trace.SpanStartOption, tags ...attribute.KeyValue) (trace.Span, context.Context) {
	sessionID, requestID, _ := validateRequest(ctx)
	spanCtx := trace.SpanContextFromContext(ctx)

	if requestID != "" {
		data, _ := base64.StdEncoding.DecodeString(requestID)
		hex := fmt.Sprintf("%032x", data)
		tid, _ := trace.TraceIDFromHex(hex)
		spanCtx = spanCtx.WithTraceID(tid)
	}

	opts = append(opts, trace.WithTimestamp(t))
	ctx, span := tracer.Start(trace.ContextWithSpanContext(ctx, spanCtx), name, opts...)
	span.SetAttributes(
		attribute.String(ProjectIDAttribute, conf.projectID),
		attribute.String(SessionIDAttribute, sessionID),
		attribute.String(RequestIDAttribute, requestID),
	)
	// prioritize values passed in tags for project, session, request IDs
	span.SetAttributes(tags...)
	return span, ctx
}

func StartTrace(ctx context.Context, name string, tags ...attribute.KeyValue) (trace.Span, context.Context) {
	return StartTraceWithTimestamp(ctx, name, time.Now(), nil, tags...)
}

func StartTraceWithoutResourceAttributes(ctx context.Context, name string, opts []trace.SpanStartOption, tags ...attribute.KeyValue) (trace.Span, context.Context) {
	resourceAttributes := []attribute.KeyValue{
		semconv.ServiceNameKey.String(""),
		semconv.ServiceVersionKey.String(""),
		semconv.ContainerIDKey.String(""),
		semconv.HostNameKey.String(""),
		semconv.OSDescriptionKey.String(""),
		semconv.OSTypeKey.String(""),
		semconv.ProcessExecutableNameKey.String(""),
		semconv.ProcessExecutablePathKey.String(""),
		semconv.ProcessOwnerKey.String(""),
		semconv.ProcessPIDKey.String(""),
		semconv.ProcessRuntimeDescriptionKey.String(""),
		semconv.ProcessRuntimeNameKey.String(""),
		semconv.ProcessRuntimeVersionKey.String(""),
	}

	attrs := append(resourceAttributes, tags...)

	return StartTraceWithTimestamp(ctx, name, time.Now(), opts, attrs...)
}

func EndTrace(span trace.Span) {
	span.End(trace.WithStackTrace(true))
}

// RecordMetric is used to record arbitrary metrics in your Go backend.
//
// Scout will process these metrics in the context of your session and expose them through charts.
//
// For example, you may want to record the latency of a database query as a metric that you can graph and monitor.
func RecordMetric(ctx context.Context, name string, value float64, tags ...attribute.KeyValue) {
	span, _ := StartTraceWithTimestamp(ctx, ScopedKey("metric", ptr.String("-")), time.Now(), []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindClient)}, tags...)
	defer EndTrace(span)
	span.AddEvent(MetricEvent, trace.WithAttributes(attribute.String(MetricEventName, name), attribute.Float64(MetricEventValue, value)))
}

// RecordError processes `err` to be recorded as a part of the session or network request.
//
// scout session and trace are inferred from the context.
//
// If sessionID is not set, then the error is associated with the project without a session context.
func RecordError(ctx context.Context, err error, tags ...attribute.KeyValue) context.Context {
	span, ctx := StartTraceWithTimestamp(ctx, ScopedKey("ctx", ptr.String("-")), time.Now(), []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindClient)}, tags...)
	defer EndTrace(span)
	RecordSpanError(span, err)
	return ctx
}

func RecordSpanError(span trace.Span, err error, tags ...attribute.KeyValue) {
	if urlErr, ok := err.(*url.Error); ok {
		span.SetAttributes(attribute.String("Op", urlErr.Op))
		span.SetAttributes(attribute.String(ErrorURLAttribute, urlErr.URL))
	}
	span.SetAttributes(tags...)
	// if this is an error with true stacktrace, then create the event directly since otel doesn't support saving a custom stacktrace
	var stackErr ErrorWithStack
	if errors.As(err, &stackErr) {
		RecordSpanErrorWithStack(span, stackErr)
	} else {
		span.RecordError(err, trace.WithStackTrace(true))
	}
}

func RecordSpanErrorWithStack(span trace.Span, err ErrorWithStack) {
	stackTrace := fmt.Sprintf("%+v", err.StackTrace())
	span.AddEvent(semconv.ExceptionEventName, trace.WithAttributes(
		semconv.ExceptionTypeKey.String(reflect.TypeOf(err).String()),
		semconv.ExceptionMessageKey.String(err.Error()),
		semconv.ExceptionStacktraceKey.String(stackTrace),
	))
}
