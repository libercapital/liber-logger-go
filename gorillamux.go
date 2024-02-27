package liberlogger

import (
	"bytes"
	"net/http"
)

func GorillaMux(routesIgnore []string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body interface{}

			ctx := r.Context()

			if ignoreRoute(routesIgnore, r) {
				next.ServeHTTP(w, r)
				return
			}

			err := extractBody(r, &body)

			logRespWriter := NewLogResponseWriter(w, r)

			if err != nil {
				Error(ctx, err).
					Interface("headers", parseHeaders(r.Header)).
					Interface("body", body).
					Dict("extra", extraLogs(r, err)).
					Msg(formatFinalMsg(r, "HTTP Server - Request | Error when parse in liberlogger"))

				next.ServeHTTP(logRespWriter, r)

				Info(ctx).
					Interface("headers", parseHeaders(logRespWriter.Header())).
					Interface("body", Redact([]string{}, []string{}, logRespWriter.buf)).
					Dict("extra", extraLogs(r, nil)).
					Msg(formatFinalMsg(r, "HTTP Server - Response |"))
				return
			}

			Info(ctx).
				Interface("headers", parseHeaders(r.Header)).
				Interface("body", body).
				Dict("extra", extraLogs(r, nil)).
				Msg(formatFinalMsg(r, "HTTP Server - Request |"))

			next.ServeHTTP(logRespWriter, r)

			Info(ctx).
				Interface("headers", parseHeaders(logRespWriter.Header())).
				Interface("body", Redact([]string{}, []string{}, logRespWriter.buf)).
				Dict("extra", extraLogs(r, nil)).
				Msg(formatFinalMsg(logRespWriter, "HTTP Server - Response |"))
		})
	}
}

func GorillaMuxRedacted(redactKeys []string, maskKeys []string, routesIgnore []string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body interface{}

			ctx := r.Context()

			if ignoreRoute(routesIgnore, r) {
				next.ServeHTTP(w, r)
				return
			}

			err := extractBody(r, &body)

			logRespWriter := NewLogResponseWriter(w, r)

			if err != nil {
				Error(ctx, err).
					Interface("headers", Redact(redactKeys, maskKeys, parseHeaders(r.Header))).
					Interface("body", Redact(redactKeys, maskKeys, body)).
					Dict("extra", extraLogs(r, err)).
					Msg(formatFinalMsg(r, "HTTP Server - Request | Error when parse in liberlogger"))

				next.ServeHTTP(logRespWriter, r)

				Info(ctx).
					Interface("headers", Redact(redactKeys, maskKeys, parseHeaders(logRespWriter.Header()))).
					Interface("body", Redact(redactKeys, maskKeys, logRespWriter.buf)).
					Dict("extra", extraLogs(r, nil)).
					Msg(formatFinalMsg(r, "HTTP Server - Response |"))
				return
			}

			Info(ctx).
				Interface("headers", Redact(redactKeys, maskKeys, parseHeaders(r.Header))).
				Interface("body", Redact(redactKeys, maskKeys, body)).
				Dict("extra", extraLogs(r, nil)).
				Msg(formatFinalMsg(r, "HTTP Server - Request |"))

			next.ServeHTTP(logRespWriter, r)

			Info(ctx).
				Interface("headers", Redact(redactKeys, maskKeys, parseHeaders(logRespWriter.Header()))).
				Interface("body", Redact(redactKeys, maskKeys, logRespWriter.buf)).
				Dict("extra", extraLogs(r, nil)).
				Msg(formatFinalMsg(logRespWriter, "HTTP Server - Response |"))
		})
	}
}

type LogResponseWriter struct {
	http.ResponseWriter
	StatusCode int
	buf        bytes.Buffer
	Request    *http.Request
}

func NewLogResponseWriter(w http.ResponseWriter, r *http.Request) *LogResponseWriter {
	return &LogResponseWriter{ResponseWriter: w, Request: r}
}

func (w *LogResponseWriter) WriteHeader(code int) {
	w.StatusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *LogResponseWriter) Write(body []byte) (int, error) {
	w.buf.Write(body)
	return w.ResponseWriter.Write(body)
}
