package tracing

import (
	"context"

	"github.com/labstack/echo/v4"
	echotrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4"
)

// echoServerSpanKey marks the ctx with the server span created by EchoV4Trace.
type echoServerSpanKey struct{}

type echoServerSpan struct {
	spanID   uint64
	resource string
	service  string
}

// EchoV4Config configures EchoV4Trace.
//
// The span's http.status_code comes from the error returned by the handler: an *echo.HTTPError gives its code and
// any other error gives 500, even when a custom HTTPErrorHandler writes another status. Return echo.NewHTTPError,
// or pass echotrace.WithErrorTranslator in Options, to keep the span status equal to the response status.
type EchoV4Config struct {
	ServiceName   string                    // Empty keeps the tracer's global service (tracing.StartTrace). Also used by the spans started inside the handlers.
	RoutesIgnore  []string                  // Routes without span nor log fields, matched against c.Path() or the request URL path.
	IgnoreRequest func(c echo.Context) bool // Also skips the request when it returns true (OR-ed with RoutesIgnore).
	Options       []echotrace.Option        // Extra echotrace options (e.g. echotrace.WithHeaderTags). An echotrace.WithIgnoreRequest here overrides RoutesIgnore and IgnoreRequest, and an echotrace.WithServiceName is not seen by the handler spans; use ServiceName instead.
}

// EchoV4Trace creates a Data Dog server span for each request, continuing the trace received in the headers,
// and fills the request context with the log fields of that span. Register it before liberlogger.EchoV4Redacted.
func EchoV4Trace(cfg EchoV4Config) echo.MiddlewareFunc {
	var opts []echotrace.Option

	if cfg.ServiceName != "" {
		opts = append(opts, echotrace.WithServiceName(cfg.ServiceName))
	}

	if len(cfg.RoutesIgnore) > 0 || cfg.IgnoreRequest != nil {
		opts = append(opts, echotrace.WithIgnoreRequest(func(c echo.Context) bool {
			for _, route := range cfg.RoutesIgnore {
				if route == c.Path() || route == c.Request().URL.Path {
					return true
				}
			}

			return cfg.IgnoreRequest != nil && cfg.IgnoreRequest(c)
		}))
	}

	opts = append(opts, cfg.Options...)

	traceMiddleware := echotrace.Middleware(opts...)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return traceMiddleware(func(c echo.Context) error {
			req := c.Request()
			ctx := req.Context()

			if span, ok := SpanFromContext(ctx); ok {
				service := cfg.ServiceName
				if service == "" {
					service = tracingParams.serviceName
				}

				ctx = context.WithValue(ctx, echoServerSpanKey{}, echoServerSpan{
					spanID:   span.Context().SpanID(),
					resource: req.Method + " " + c.Path(),
					service:  service,
				})

				c.SetRequest(req.WithContext(AddTraceAndSpanToLog(ctx)))
			}

			return next(c)
		})
	}
}
