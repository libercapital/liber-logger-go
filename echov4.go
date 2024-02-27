package liberlogger

import (
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

func EchoV4(routesIgnore []string) func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var body interface{}

			ctx := c.Request().Context()

			if ignoreRoute(routesIgnore, c.Request()) {
				return next(c)
			}

			err := extractBody(c.Request(), &body)

			if err != nil {
				Error(ctx, err).
					Interface("headers", parseHeaders(c.Request().Header)).
					Interface("body", Redact([]string{}, []string{}, body)).
					Dict("extra", extraLogs(c.Request(), err)).
					Msg(formatFinalMsg(c.Request(), "HTTP Server | Error when parse in liberlogger"))

				return next(c)
			}

			Info(ctx).
				Interface("headers", parseHeaders(c.Request().Header)).
				Interface("body", Redact([]string{}, []string{}, body)).
				Dict("extra", extraLogs(c.Request(), nil)).
				Msg(formatFinalMsg(c.Request(), "HTTP Server"))

			return next(c)
		}
	}
}

func EchoV4Redacted(redactKeys []string, maskKeys []string, routesIgnore []string) func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var body interface{}

			ctx := log.Logger.WithContext(c.Request().Context())

			if ignoreRoute(routesIgnore, c.Request()) {
				return next(c)
			}

			err := extractBody(c.Request(), &body)

			if err != nil {
				Error(ctx, err).
					Interface("headers", Redact(redactKeys, maskKeys, parseHeaders(c.Request().Header))).
					Interface("body", Redact(redactKeys, maskKeys, body)).
					Dict("extra", extraLogs(c.Request(), err)).
					Msg(formatFinalMsg(c.Request(), "HTTP Server | Error when parse in liberlogger"))

				return next(c)
			}

			Info(ctx).
				Interface("headers", Redact(redactKeys, maskKeys, parseHeaders(c.Request().Header))).
				Interface("body", Redact(redactKeys, maskKeys, body)).
				Dict("extra", extraLogs(c.Request(), nil)).
				Msg(formatFinalMsg(c.Request(), "HTTP Server"))

			return next(c)
		}
	}
}
