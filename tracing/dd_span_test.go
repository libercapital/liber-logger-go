package tracing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/libercapital/liber-logger-go"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/mocktracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func requireLogFieldsOf(t *testing.T, ctx context.Context, span ddtrace.Span) {
	t.Helper()

	fields, ok := ctx.Value(liberlogger.LogFieldsKey{}).(map[string]interface{})
	if !ok {
		t.Fatal("expected log fields in ctx")
	}

	if fields["dd.span_id"] != span.Context().SpanID() {
		t.Errorf("dd.span_id = %v, want %d", fields["dd.span_id"], span.Context().SpanID())
	}

	if fields["dd.trace_id"] != span.Context().TraceID() {
		t.Errorf("dd.trace_id = %v, want %d", fields["dd.trace_id"], span.Context().TraceID())
	}
}

func splitServerAndChild(t *testing.T, mt mocktracer.Tracer) (mocktracer.Span, mocktracer.Span) {
	t.Helper()

	spans := mt.FinishedSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 finished spans, got %d", len(spans))
	}

	if spans[0].Tag(ext.SpanKind) == ext.SpanKindServer {
		return spans[0], spans[1]
	}

	return spans[1], spans[0]
}

func TestStartContextAndSpanInsideEchoV4Trace(t *testing.T) {
	tests := []struct {
		name          string
		config        SpanConfig
		handlerErr    error
		wantStatus    int
		wantOperation string
		wantResource  string
	}{
		{
			name:          "empty config uses echo defaults",
			config:        SpanConfig{},
			wantStatus:    http.StatusCreated,
			wantOperation: "echo.handler",
			wantResource:  "GET /users/:id",
		},
		{
			name:          "explicit names are kept",
			config:        SpanConfig{OperationName: "user.get", ResourceName: "get user"},
			wantStatus:    http.StatusCreated,
			wantOperation: "user.get",
			wantResource:  "get user",
		},
		{
			name:          "handler error",
			config:        SpanConfig{},
			handlerErr:    errors.New("boom"),
			wantStatus:    http.StatusInternalServerError,
			wantOperation: "echo.handler",
			wantResource:  "GET /users/:id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := startMockTracer(t)
			logs := captureLogs(t)

			var handlerCtx context.Context
			var handlerSpan ddtrace.Span

			e := newTracedEcho(EchoV4Config{}, http.MethodGet, "/users/:id", func(c echo.Context) error {
				ctx, span := StartContextAndSpan(c.Request().Context(), tt.config)
				defer span.Finish()

				handlerCtx, handlerSpan = ctx, span

				// A nested call with the child ctx reuses the child span, as before.
				if _, nested := StartContextAndSpan(ctx, SpanConfig{}); nested.Context().SpanID() != span.Context().SpanID() {
					t.Error("nested StartContextAndSpan did not reuse the child span")
				}

				if tt.handlerErr != nil {
					return tt.handlerErr
				}

				return c.NoContent(http.StatusCreated)
			})

			serve(e, httptest.NewRequest(http.MethodGet, "/users/42", nil))

			server, child := splitServerAndChild(t, mt)

			if child.SpanID() != handlerSpan.Context().SpanID() {
				t.Fatalf("returned span %d is not the finished child %d", handlerSpan.Context().SpanID(), child.SpanID())
			}

			if child.ParentID() != server.SpanID() {
				t.Errorf("child parent id = %d, want server span id %d", child.ParentID(), server.SpanID())
			}

			if child.TraceID() != server.TraceID() {
				t.Errorf("child trace id = %d, want %d", child.TraceID(), server.TraceID())
			}

			if child.OperationName() != tt.wantOperation {
				t.Errorf("child operation = %q, want %q", child.OperationName(), tt.wantOperation)
			}

			if got := child.Tag(ext.ResourceName); got != tt.wantResource {
				t.Errorf("child resource = %v, want %q", got, tt.wantResource)
			}

			if got := server.Tag(ext.HTTPCode); got != strconv.Itoa(tt.wantStatus) {
				t.Errorf("server http.status_code = %v, want %d", got, tt.wantStatus)
			}

			requireLogFieldsOf(t, handlerCtx, handlerSpan)

			// The HTTP Server logs of EchoV4Redacted keep pointing to the server span, not to the child.
			lines := parseLogLines(t, logs)
			if len(lines) != 2 {
				t.Fatalf("expected 2 log lines (request and response), got %d: %v", len(lines), lines)
			}

			for i, line := range lines {
				if got := line["dd.span_id"]; got != json.Number(strconv.FormatUint(server.SpanID(), 10)) {
					t.Errorf("log %d dd.span_id = %v, want server span id %d", i, got, server.SpanID())
				}
			}
		})
	}
}

func TestStartContextAndSpanOutsideEchoV4Trace(t *testing.T) {
	t.Run("empty ctx starts a root span", func(t *testing.T) {
		mt := startMockTracer(t)

		ctx, span := StartContextAndSpan(context.Background(), SpanConfig{OperationName: "invoice.cmd", ResourceName: "create"})
		span.Finish()

		spans := mt.FinishedSpans()
		if len(spans) != 1 {
			t.Fatalf("expected 1 finished span, got %d", len(spans))
		}

		if spans[0].ParentID() != 0 {
			t.Errorf("parent id = %d, want 0", spans[0].ParentID())
		}

		if spans[0].OperationName() != "invoice.cmd" {
			t.Errorf("operation = %q, want invoice.cmd", spans[0].OperationName())
		}

		requireLogFieldsOf(t, ctx, span)
	})

	t.Run("empty ctx with TraceID continues that trace", func(t *testing.T) {
		mt := startMockTracer(t)

		ctx, span := StartContextAndSpan(context.Background(), SpanConfig{OperationName: "invoice.cmd", TraceID: 99})
		span.Finish()

		if spans := mt.FinishedSpans(); len(spans) != 2 {
			t.Fatalf("expected 2 finished spans, got %d", len(spans))
		}

		if span.Context().TraceID() != 99 {
			t.Errorf("trace id = %d, want 99", span.Context().TraceID())
		}

		if got := span.(mocktracer.Span).ParentID(); got != 99 {
			t.Errorf("parent id = %d, want 99", got)
		}

		requireLogFieldsOf(t, ctx, span)
	})

	t.Run("ctx with a span that is not an echo server span reuses it", func(t *testing.T) {
		mt := startMockTracer(t)

		parent := tracer.StartSpan("amqp.consume")
		ctx, span := StartContextAndSpan(tracer.ContextWithSpan(context.Background(), parent), SpanConfig{OperationName: "invoice.cmd"})

		if span.Context().SpanID() != parent.Context().SpanID() {
			t.Errorf("span id = %d, want the ctx span %d", span.Context().SpanID(), parent.Context().SpanID())
		}

		span.Finish()

		if spans := mt.FinishedSpans(); len(spans) != 1 {
			t.Errorf("expected 1 finished span, got %d", len(spans))
		}

		requireLogFieldsOf(t, ctx, parent)
	})
}

// setTracingService replaces the StartTrace service. Tests using it must not run in parallel.
func setTracingService(t *testing.T, service string) {
	t.Helper()

	previous := tracingParams.serviceName
	tracingParams.serviceName = service

	t.Cleanup(func() { tracingParams.serviceName = previous })
}

func TestSpansInsideEchoV4TraceUseServerService(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		wantService string
	}{
		{name: "custom ServiceName", serviceName: "custom-api", wantService: "custom-api"},
		{name: "empty ServiceName uses the StartTrace service", serviceName: "", wantService: "global-service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setTracingService(t, "global-service")
			mt := startMockTracer(t)
			captureLogs(t)

			e := newTracedEcho(EchoV4Config{ServiceName: tt.serviceName}, http.MethodGet, "/users/:id", func(c echo.Context) error {
				ctx, child := StartContextAndSpan(c.Request().Context(), SpanConfig{})
				defer child.Finish()

				grandchild, _ := StartSpanFromContext(ctx, SpanConfig{OperationName: "user.load"})
				grandchild.Finish()

				return c.NoContent(http.StatusOK)
			})

			serve(e, httptest.NewRequest(http.MethodGet, "/users/42", nil))

			spans := mt.FinishedSpans()
			if len(spans) != 3 {
				t.Fatalf("expected 3 finished spans, got %d", len(spans))
			}

			for _, span := range spans {
				// With an empty ServiceName the server span gets the tracer's global service, which the mocktracer
				// does not set, so only the handler spans are checked.
				if span.Tag(ext.SpanKind) == ext.SpanKindServer && tt.serviceName == "" {
					continue
				}

				if got := span.Tag(ext.ServiceName); got != tt.wantService {
					t.Errorf("%s service = %v, want %q", span.OperationName(), got, tt.wantService)
				}
			}
		})
	}
}

func TestSpansOutsideEchoV4TraceUseStartTraceService(t *testing.T) {
	setTracingService(t, "global-service")
	mt := startMockTracer(t)

	ctx, span := StartContextAndSpan(context.Background(), SpanConfig{OperationName: "invoice.cmd"})
	child, _ := StartSpanFromContext(ctx, SpanConfig{OperationName: "invoice.save"})
	child.Finish()
	span.Finish()

	spans := mt.FinishedSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 finished spans, got %d", len(spans))
	}

	for _, finished := range spans {
		if got := finished.Tag(ext.ServiceName); got != "global-service" {
			t.Errorf("%s service = %v, want global-service", finished.OperationName(), got)
		}
	}
}
