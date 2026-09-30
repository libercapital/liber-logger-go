package tracing

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/libercapital/liber-logger-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/mocktracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// captureLogs redirects the global logger to a buffer. Tests using it must not run in parallel.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	previousLogger := log.Logger
	previousLevel := zerolog.GlobalLevel()

	log.Logger = zerolog.New(buf)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	t.Cleanup(func() {
		log.Logger = previousLogger
		zerolog.SetGlobalLevel(previousLevel)
	})

	return buf
}

// parseLogLines keeps numbers as json.Number, so 64 bits span and trace ids are not rounded.
func parseLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var lines []map[string]any

	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}

		decoder := json.NewDecoder(strings.NewReader(line))
		decoder.UseNumber()

		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("invalid log line %q: %v", line, err)
		}

		lines = append(lines, entry)
	}

	return lines
}

// startMockTracer replaces the global tracer. Tests using it must not run in parallel.
func startMockTracer(t *testing.T) mocktracer.Tracer {
	t.Helper()

	mt := mocktracer.Start()
	t.Cleanup(mt.Stop)

	return mt
}

func newTracedEcho(cfg EchoV4Config, method string, path string, handler echo.HandlerFunc) *echo.Echo {
	e := echo.New()
	e.Logger.SetOutput(io.Discard)
	e.Use(EchoV4Trace(cfg), liberlogger.EchoV4Redacted(liberlogger.DefaultKeys, liberlogger.DefaultKeysToMask, []string{"/health"}))
	e.Add(method, path, handler)

	return e
}

func serve(e *echo.Echo, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec
}

func requireOneSpan(t *testing.T, mt mocktracer.Tracer) mocktracer.Span {
	t.Helper()

	spans := mt.FinishedSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 finished span, got %d", len(spans))
	}

	return spans[0]
}

func TestEchoV4TraceServerSpan(t *testing.T) {
	tests := []struct {
		name       string
		handler    echo.HandlerFunc
		wantStatus int
		wantBody   string
	}{
		{
			name:       "200",
			handler:    func(c echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")}) },
			wantStatus: http.StatusOK,
			wantBody:   `{"id":"42"}`,
		},
		{
			name:       "400 returned as echo.HTTPError",
			handler:    func(c echo.Context) error { return echo.NewHTTPError(http.StatusBadRequest, "invalid id") },
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"message":"invalid id"}`,
		},
		{
			name: "400 written with c.JSON",
			handler: func(c echo.Context) error {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"invalid id"}`,
		},
		{
			name:       "500 from errors.New",
			handler:    func(c echo.Context) error { return errors.New("boom") },
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"message":"Internal Server Error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := startMockTracer(t)
			captureLogs(t)

			e := newTracedEcho(EchoV4Config{ServiceName: "my-service"}, http.MethodGet, "/users/:id", tt.handler)
			rec := serve(e, httptest.NewRequest(http.MethodGet, "/users/42", nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			// The error response is written once, by c.Error inside EchoV4Redacted.
			if got := strings.TrimSpace(rec.Body.String()); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}

			span := requireOneSpan(t, mt)

			if span.OperationName() != "http.request" {
				t.Errorf("operation = %q, want http.request", span.OperationName())
			}

			if got := span.Tag(ext.ResourceName); got != "GET /users/:id" {
				t.Errorf("resource = %v, want GET /users/:id", got)
			}

			if got := span.Tag(ext.SpanKind); got != ext.SpanKindServer {
				t.Errorf("span.kind = %v, want %s", got, ext.SpanKindServer)
			}

			if got := span.Tag(ext.HTTPCode); got != strconv.Itoa(tt.wantStatus) {
				t.Errorf("http.status_code = %v, want %d", got, tt.wantStatus)
			}

			if got := span.Tag(ext.ServiceName); got != "my-service" {
				t.Errorf("service = %v, want my-service", got)
			}
		})
	}
}

func TestEchoV4TraceLogsCarryServerSpan(t *testing.T) {
	mt := startMockTracer(t)
	logs := captureLogs(t)

	e := newTracedEcho(EchoV4Config{}, http.MethodPost, "/users", func(c echo.Context) error {
		return c.JSON(http.StatusCreated, map[string]string{"status": "created"})
	})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"john"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	serve(e, req)

	span := requireOneSpan(t, mt)
	lines := parseLogLines(t, logs)

	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines (request and response), got %d: %v", len(lines), lines)
	}

	for i, line := range lines {
		if !strings.HasPrefix(line["message"].(string), "HTTP Server") {
			t.Errorf("log %d message = %q, want HTTP Server prefix", i, line["message"])
		}

		if got := line["dd.trace_id"]; got != json.Number(strconv.FormatUint(span.TraceID(), 10)) {
			t.Errorf("log %d dd.trace_id = %v, want %d", i, got, span.TraceID())
		}

		if got := line["dd.span_id"]; got != json.Number(strconv.FormatUint(span.SpanID(), 10)) {
			t.Errorf("log %d dd.span_id = %v, want %d", i, got, span.SpanID())
		}
	}
}

func TestEchoV4TraceContinuesIncomingTrace(t *testing.T) {
	mt := startMockTracer(t)
	captureLogs(t)

	e := newTracedEcho(EchoV4Config{}, http.MethodGet, "/users/:id", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	req.Header.Set(tracer.DefaultTraceIDHeader, "123")
	req.Header.Set(tracer.DefaultParentIDHeader, "456")
	serve(e, req)

	span := requireOneSpan(t, mt)

	if span.TraceID() != 123 {
		t.Errorf("trace id = %d, want 123", span.TraceID())
	}

	if span.ParentID() != 456 {
		t.Errorf("parent id = %d, want 456", span.ParentID())
	}
}

func TestEchoV4TraceRoutesIgnore(t *testing.T) {
	tests := []struct {
		name  string
		route string
		path  string
	}{
		{name: "request path", route: "/health", path: "/health"},
		{name: "route path", route: "/users/:id", path: "/users/42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := startMockTracer(t)
			captureLogs(t)

			handlerCalled := false

			e := newTracedEcho(EchoV4Config{RoutesIgnore: []string{tt.route}}, http.MethodGet, tt.route, func(c echo.Context) error {
				handlerCalled = true

				if fields := c.Request().Context().Value(liberlogger.LogFieldsKey{}); fields != nil {
					t.Errorf("expected no log fields in ctx, got %v", fields)
				}

				return c.NoContent(http.StatusOK)
			})

			rec := serve(e, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if !handlerCalled {
				t.Fatal("handler was not called")
			}

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rec.Code)
			}

			if spans := mt.FinishedSpans(); len(spans) != 0 {
				t.Errorf("expected no spans, got %d", len(spans))
			}
		})
	}
}

func TestEchoV4TraceIgnoreRequestIsCombinedWithRoutesIgnore(t *testing.T) {
	cfg := EchoV4Config{
		RoutesIgnore:  []string{"/health"},
		IgnoreRequest: func(c echo.Context) bool { return c.Request().Header.Get("X-Skip-Trace") == "true" },
	}

	tests := []struct {
		name      string
		path      string
		skip      bool
		wantSpans int
	}{
		{name: "ignored by RoutesIgnore", path: "/health", wantSpans: 0},
		{name: "ignored by IgnoreRequest", path: "/users/42", skip: true, wantSpans: 0},
		{name: "traced", path: "/users/42", wantSpans: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := startMockTracer(t)
			captureLogs(t)

			e := newTracedEcho(cfg, http.MethodGet, "/users/:id", func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			})
			e.GET("/health", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.skip {
				req.Header.Set("X-Skip-Trace", "true")
			}

			serve(e, req)

			if spans := mt.FinishedSpans(); len(spans) != tt.wantSpans {
				t.Errorf("expected %d spans, got %d", tt.wantSpans, len(spans))
			}
		})
	}
}

func TestEchoV4TraceEmptyServiceNameUsesGlobalService(t *testing.T) {
	t.Setenv("DD_TRACE_STARTUP_LOGS", "false")
	t.Setenv("DD_INSTRUMENTATION_TELEMETRY_ENABLED", "false")
	t.Setenv("DD_REMOTE_CONFIGURATION_ENABLED", "false")

	t.Setenv("DD_SERVICE", "")
	t.Setenv("OTEL_SERVICE_NAME", "")

	// StartTrace sets the tracer's global service, which is kept after the real tracer is replaced by the mock
	// (mocktracer does not read DD_SERVICE). The cleanup restarts the real tracer without a service, which resets
	// the global service to empty, so later tests keep the "echo" default.
	StartTrace("global-service", "test", tracer.WithAgentAddr("127.0.0.1:1"), tracer.WithLogger(discardLogger{}))
	StopTrace()
	t.Cleanup(func() {
		StartTrace("", "test", tracer.WithAgentAddr("127.0.0.1:1"), tracer.WithLogger(discardLogger{}))
		StopTrace()
	})

	mt := startMockTracer(t)
	captureLogs(t)

	e := newTracedEcho(EchoV4Config{}, http.MethodGet, "/users/:id", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	serve(e, httptest.NewRequest(http.MethodGet, "/users/42", nil))

	span := requireOneSpan(t, mt)

	if got := span.Tag(ext.ServiceName); got != "global-service" {
		t.Errorf("service = %v, want global-service", got)
	}
}

func TestEchoV4TracePropagatesToHttpTraceClient(t *testing.T) {
	mt := startMockTracer(t)
	captureLogs(t)

	var receivedTraceID string

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceID = r.Header.Get(tracer.DefaultTraceIDHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer downstream.Close()

	client := HttpTrace(&http.Client{}, HttpTraceConfig{
		OperationName: "http.client.request",
		ResourceName:  func(req *http.Request) string { return req.Method + " downstream" },
	})

	e := newTracedEcho(EchoV4Config{}, http.MethodGet, "/users/:id", func(c echo.Context) error {
		req, err := http.NewRequestWithContext(c.Request().Context(), http.MethodGet, downstream.URL, nil)
		if err != nil {
			return err
		}

		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		return c.NoContent(http.StatusOK)
	})

	serve(e, httptest.NewRequest(http.MethodGet, "/users/42", nil))

	var server mocktracer.Span

	for _, span := range mt.FinishedSpans() {
		if span.Tag(ext.SpanKind) == ext.SpanKindServer {
			server = span
		}
	}

	if server == nil {
		t.Fatal("server span not found")
	}

	if receivedTraceID != strconv.FormatUint(server.TraceID(), 10) {
		t.Errorf("downstream %s = %q, want %d", tracer.DefaultTraceIDHeader, receivedTraceID, server.TraceID())
	}
}

type discardLogger struct{}

func (discardLogger) Log(string) {}
