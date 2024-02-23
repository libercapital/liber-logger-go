package liberlogger

import (
	"context"

	"github.com/google/uuid"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type StartContextAndSpanConfig struct {
	ServiceName   string          // ServiceName refers to the service name.
	OperationName string          // OperationName refers to the current operation.
	Ctx           context.Context // Ctx refers to an optional field. If a context is given, it will be used as a parent for the returned context, if is not, a new context will be created.
}

// StartContextAndSpan creates and retrieves a new context, filled with the log fields. Also start and return a new data dog span,
// but is required the existence of a previous tracer, otherwise a noop span will be generated instead.
func StartContextAndSpan(confg StartContextAndSpanConfig) (context.Context, ddtrace.Span) {
	var ctx context.Context

	if confg.Ctx != nil {
		ctx = confg.Ctx
	} else {
		ctx = context.Background()
	}

	span, exist := tracer.SpanFromContext(ctx)
	if !exist {
		span, ctx = tracer.StartSpanFromContext(
			ctx, confg.OperationName,
			tracer.ServiceName(confg.ServiceName),
		)
	}

	logFields := map[string]interface{}{
		"span_id":  span.Context().SpanID(),
		"trace_id": span.Context().TraceID(),
		"log_id":   uuid.NewString(),
	}

	return context.WithValue(ctx, LogFieldsKey{}, logFields), span
}
