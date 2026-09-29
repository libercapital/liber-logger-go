package liberlogger

import (
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

func EchoV4(routesIgnore []string) func(next echo.HandlerFunc) echo.HandlerFunc {
	return echoV4Middleware([]string{}, []string{}, routesIgnore)
}

func EchoV4Redacted(redactKeys []string, maskKeys []string, routesIgnore []string) func(next echo.HandlerFunc) echo.HandlerFunc {
	return echoV4Middleware(redactKeys, maskKeys, routesIgnore)
}

func echoV4Middleware(redactKeys []string, maskKeys []string, routesIgnore []string) func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var body interface{}

			req := c.Request()
			ctx := log.Logger.WithContext(req.Context())

			if ignoreRoute(routesIgnore, req) {
				return next(c)
			}

			if err := extractBody(req, &body); err != nil {
				Error(ctx, err).
					Interface("headers", Redact(redactKeys, maskKeys, parseHeaders(req.Header))).
					Interface("body", Redact(redactKeys, maskKeys, body)).
					Dict("extra", extraLogs(req, err)).
					Msg(formatFinalMsg(req, "HTTP Server | Error when parse in liberlogger"))
			} else {
				Info(ctx).
					Interface("headers", Redact(redactKeys, maskKeys, parseHeaders(req.Header))).
					Interface("body", Redact(redactKeys, maskKeys, body)).
					Dict("extra", extraLogs(req, nil)).
					Msg(formatFinalMsg(req, "HTTP Server"))
			}

			res := c.Response()
			logRespWriter := NewLogResponseWriter(res.Writer, req)
			res.Writer = logRespWriter

			err := next(c)

			// The response of a handler error is only written by the HTTPErrorHandler, so it must
			// run before logging. The error is still returned (same contract as echo's RequestLogger
			// with HandleError) and the default handler ignores it because the response is committed.
			if err != nil {
				c.Error(err)
			}

			logRespWriter.StatusCode = res.Status

			Info(ctx).
				Interface("headers", Redact(redactKeys, maskKeys, parseHeaders(res.Header()))).
				Interface("body", responseBody(&logRespWriter.buf, redactKeys, maskKeys)).
				Dict("extra", extraLogs(logRespWriter, err)).
				Msg(formatFinalMsg(logRespWriter, "HTTP Server"))

			return err
		}
	}
}
