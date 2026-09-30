package tracing

import (
	"context"
	"math"
	"net/http"

	"github.com/google/uuid"
	"github.com/libercapital/liber-logger-go"
	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type SpanTags map[string]interface{}
type SpanConfig StartContextAndSpanConfig[string, SpanTags]

func (t SpanTags) toSpanTag(opts *[]ddtrace.StartSpanOption) {
	for key, value := range t {
		*opts = append(*opts, tracer.Tag(key, value))
	}
}

type ResourceNameInterface interface {
	string | func(req *http.Request) string | func(router *muxtrace.Router, req *http.Request) string
}

type TagsInterface interface {
	SpanTags | HttpTraceTags | GorillaMuxTags
}

type StartContextAndSpanConfig[T ResourceNameInterface, N TagsInterface] struct {
	OperationName string // OperationName refers to the current operation.
	SpanType      string // It is necessary to utilize package https://pkg.go.dev/gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext to use the types correctly.
	ResourceName  T
	TraceID       uint64
	Tags          N
	AnalyticsRate float64
}

// StartContextAndSpan creates and retrieves a new context, filled with the log fields. Also start and return a new data dog span,
// but is required the existence of a previous tracer, otherwise a noop span will be generated instead.
//
// When the ctx already has a span, that span is returned and reused, except for the server span created by EchoV4Trace:
// inside its handlers a child span is started, so the handler's span.Finish() does not finish the server span.
// For that child, an empty OperationName becomes "echo.handler" and an empty ResourceName becomes the server resource.
// The returned span must always be finished there, otherwise the whole request trace is not flushed.
// Spans started in a ctx of an EchoV4Trace request use the service of its server span.
func StartContextAndSpan(ctx context.Context, traceConfig SpanConfig) (context.Context, ddtrace.Span) {
	echoServer, inEchoServer := echoServerSpanFromContext(ctx)
	if inEchoServer {
		if traceConfig.OperationName == "" {
			traceConfig.OperationName = "echo.handler"
		}

		if traceConfig.ResourceName == "" {
			traceConfig.ResourceName = echoServer.resource
		}
	}

	opts := []ddtrace.StartSpanOption{
		tracer.ServiceName(serviceFromContext(ctx)),
		tracer.SpanType(traceConfig.SpanType),
		tracer.ResourceName(traceConfig.ResourceName),
	}

	if !math.IsNaN(traceConfig.AnalyticsRate) {
		opts = append(opts, tracer.AnalyticsRate(traceConfig.AnalyticsRate))
	}

	traceConfig.Tags.toSpanTag(&opts)

	span, exist := tracer.SpanFromContext(ctx)
	if inEchoServer {
		span, ctx = tracer.StartSpanFromContext(ctx, traceConfig.OperationName, opts...)
	} else if !exist {
		if traceConfig.TraceID > 0 {
			childOfTrace := tracer.StartSpan(
				traceConfig.OperationName,
				tracer.WithSpanID(traceConfig.TraceID),
			)

			childOfTrace.Finish()

			opts = append(opts, tracer.ChildOf(childOfTrace.Context()))
		}

		span, ctx = tracer.StartSpanFromContext(
			ctx,
			traceConfig.OperationName,
			opts...,
		)
	}

	logFields := map[string]interface{}{
		"dd.span_id":  span.Context().SpanID(),
		"dd.trace_id": span.Context().TraceID(),
		"log_id":      uuid.NewString(),
	}

	return context.WithValue(ctx, liberlogger.LogFieldsKey{}, logFields), span
}

// echoServerSpanFromContext reports whether the current span of the ctx is the server span created by EchoV4Trace.
func echoServerSpanFromContext(ctx context.Context) (echoServerSpan, bool) {
	server, ok := ctx.Value(echoServerSpanKey{}).(echoServerSpan)
	if !ok {
		return echoServerSpan{}, false
	}

	span, exist := tracer.SpanFromContext(ctx)

	return server, exist && span.Context().SpanID() == server.spanID
}

// serviceFromContext returns the service of the EchoV4Trace server span of the ctx, or the StartTrace service outside it.
func serviceFromContext(ctx context.Context) string {
	if server, ok := ctx.Value(echoServerSpanKey{}).(echoServerSpan); ok {
		return server.service
	}

	return tracingParams.serviceName
}

func SpanFromContext(ctx context.Context) (ddtrace.Span, bool) {
	return tracer.SpanFromContext(ctx)
}

func StartSpanFromContext(ctx context.Context, traceConfig SpanConfig) (ddtrace.Span, context.Context) {
	opts := []ddtrace.StartSpanOption{
		tracer.ServiceName(serviceFromContext(ctx)),
		tracer.SpanType(traceConfig.SpanType),
		tracer.ResourceName(traceConfig.ResourceName),
	}

	traceConfig.Tags.toSpanTag(&opts)

	span, ctx := tracer.StartSpanFromContext(ctx, traceConfig.OperationName, opts...)

	logFields := map[string]interface{}{
		"dd.span_id":  span.Context().SpanID(),
		"dd.trace_id": span.Context().TraceID(),
		"log_id":      uuid.NewString(),
	}

	return span, context.WithValue(ctx, liberlogger.LogFieldsKey{}, logFields)
}

func AddTraceAndSpanToLog(ctx context.Context) context.Context {
	if span, ok := SpanFromContext(ctx); ok {
		logFields := map[string]interface{}{
			"dd.span_id":  span.Context().SpanID(),
			"dd.trace_id": span.Context().TraceID(),
			"log_id":      uuid.NewString(),
		}

		ctx = context.WithValue(ctx, liberlogger.LogFieldsKey{}, logFields)
	}

	return ctx
}
