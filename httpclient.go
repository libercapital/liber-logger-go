package liberlogger

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

// This type implements the http.RoundTripper interface
type HttpClient struct {
	Proxied      http.RoundTripper
	RedactedKeys []string
	Maskedkeys   []string
}

func (hc HttpClient) RoundTrip(req *http.Request) (res *http.Response, err error) {
	var bodyRequest interface{}

	extractBody(req, &bodyRequest)

	ctx := log.Logger.WithContext(req.Context())

	log.
		Ctx(ctx).
		Info().
		Interface("headers", Redact(hc.RedactedKeys, hc.Maskedkeys, parseHeaders(req.Header))).
		Interface("body", Redact(hc.RedactedKeys, hc.Maskedkeys, bodyRequest)).
		Dict("extra", extraLogs(req, nil)).
		Msg(formatFinalMsg(req, "HTTP Client"))

	res, err = hc.Proxied.RoundTrip(req)

	if err != nil {
		log.
			Ctx(ctx).
			Error().
			Interface("headers", Redact(hc.RedactedKeys, hc.Maskedkeys, parseHeaders(req.Header))).
			Interface("body", Redact(hc.RedactedKeys, hc.Maskedkeys, bodyRequest)).
			Dict("extra", extraLogs(req, err)).
			Msg(formatFinalMsg(req, "HTTP Client"))
	} else {
		var bodyResponse interface{}

		extractBody(res, &bodyResponse)

		log.
			Ctx(ctx).
			Info().
			Interface("headers", Redact(hc.RedactedKeys, hc.Maskedkeys, parseHeaders(res.Header))).
			Interface("body", Redact(hc.RedactedKeys, hc.Maskedkeys, bodyResponse)).
			Dict("extra", extraLogs(res, nil)).
			Msg(formatFinalMsg(res, "HTTP Client"))
	}

	return
}

func (hc *HttpClient) AddKeysToMask(keys []string){
	if hc.Maskedkeys != nil && keys != nil {
		hc.Maskedkeys = append(hc.Maskedkeys, keys...)
		return
	}
}

func (hc *HttpClient) AddKeysToRedact(keys []string){
	if hc.RedactedKeys != nil && keys != nil {
		hc.RedactedKeys = append(hc.RedactedKeys, keys...)
		return
	}
}
