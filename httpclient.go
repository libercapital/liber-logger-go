package liberlogger

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// This type implements the http.RoundTripper interface
type HttpClient struct {
	Proxied      http.RoundTripper
	RedactedKeys []string
	Maskedkeys   []string
}

func (hc HttpClient) getRequestBody(req *http.Request) any {
	var bodyRequest map[string]any

	err := extractBody(req, &bodyRequest)

	if err != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		defer req.Body.Close()

		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		return string(bodyBytes)
	}

	return Redact(hc.RedactedKeys, hc.Maskedkeys, bodyRequest)
}

func (hc HttpClient) getResponseBody(res *http.Response) any {
	var bodyResponse map[string]any

	err := extractBody(res, &bodyResponse)

	if err != nil {
		bodyBytes, _ := io.ReadAll(res.Body)

		defer res.Body.Close()

		res.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		return string(bodyBytes)
	}

	return Redact(hc.RedactedKeys, hc.Maskedkeys, bodyResponse)
}

func (hc HttpClient) RoundTrip(req *http.Request) (res *http.Response, err error) {
	ctx := context.TODO()

	requestBody := hc.getRequestBody(req)

	Info(ctx).
		Interface("headers", Redact(hc.RedactedKeys, hc.Maskedkeys, parseHeaders(req.Header))).
		Interface("body", requestBody).
		Dict("extra", extraLogs(req, nil)).
		Msg(formatFinalMsg(req, "HTTP Client"))

	res, err = hc.Proxied.RoundTrip(req)

	if err != nil {
		responseBody := hc.getResponseBody(res)

		Error(ctx, err).
			Interface("headers", Redact(hc.RedactedKeys, hc.Maskedkeys, parseHeaders(req.Header))).
			Interface("body", responseBody).
			Dict("extra", extraLogs(req, err)).
			Msg(formatFinalMsg(req, "HTTP Client"))

		return
	}

	responseBody := hc.getResponseBody(res)

	Info(ctx).
		Interface("headers", Redact(hc.RedactedKeys, hc.Maskedkeys, parseHeaders(res.Header))).
		Interface("body", responseBody).
		Dict("extra", extraLogs(res, nil)).
		Msg(formatFinalMsg(res, "HTTP Client"))

	return
}

func (hc *HttpClient) AddKeysToMask(keys []string) {
	if hc.Maskedkeys != nil && keys != nil {
		hc.Maskedkeys = append(hc.Maskedkeys, keys...)
		return
	}
}

func (hc *HttpClient) AddKeysToRedact(keys []string) {
	if hc.RedactedKeys != nil && keys != nil {
		hc.RedactedKeys = append(hc.RedactedKeys, keys...)
		return
	}
}
