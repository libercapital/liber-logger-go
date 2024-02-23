package liberlogger

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

const (
	REDACTED = "REDACTED"
)

var DefaultKeys = []string{
	"access_token",
	"client_secret",
	"Authorization",
	"password",
}

var DefaultKeysToMask = []string{
	"document",
	"cpf",
	"cnpj",
}

func extraLogs(request interface{}, err error) *zerolog.Event {
	log := zerolog.Dict()

	if err != nil {
		log.Interface("error", err.Error())
	}

	switch request := request.(type) {
	case *http.Request:
		log.
			Interface("url", request.URL.String()).
			Interface("method", request.Method)
	case *http.Response:
		log.
			Interface("request", map[string]interface{}{
				"url":    request.Request.URL.String(),
				"method": request.Request.Method,
			})
	}
	return log
}

func formatFinalMsg(request interface{}, msg string) (finalMsg string) {
	switch request := request.(type) {
	case *http.Request:
		finalMsg = fmt.Sprintf("%s %s %s",
			msg,
			request.Method,
			request.URL.String(),
		)
	case *http.Response:
		finalMsg = fmt.Sprintf("%s %s %d %s",
			msg,
			request.Request.Method,
			request.StatusCode,
			request.Request.URL.String(),
		)
	case *LogResponseWriter:
		finalMsg = fmt.Sprintf("%s %s %d %s",
			msg,
			request.Request.Method,
			request.StatusCode,
			request.Request.URL.String(),
		)
	}

	return
}

func ignoreRoute(ignoreList []string, request *http.Request) bool {
	for _, ignore := range ignoreList {
		if strings.Compare(ignore, request.RequestURI) == 0 {
			return true
		}
	}

	return false
}
