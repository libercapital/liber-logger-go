package liberlogger

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"reflect"

	"github.com/kataras/compress"
	"github.com/rs/zerolog"
)

func parseHeaders(headers map[string][]string) map[string]string {
	newHeaders := map[string]string{}

	for key, header := range headers {
		newHeaders[key] = header[0]
	}

	return newHeaders
}

func extractBody(request interface{}, parseTo interface{}) error {
	var bodyBytes []byte

	typeParse := reflect.ValueOf(parseTo)

	if typeParse.Kind() != reflect.Ptr {
		return errors.New("parseTo must be a pointer")
	}

	switch request := request.(type) {
	case *http.Request:
		var err error

		if request.Body == nil {
			return nil
		}

		bodyBytes, err = ioutil.ReadAll(request.Body)

		if err != nil {
			return err
		}

		defer request.Body.Close()

		request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	case *http.Response:
		var err error

		if request.Body == nil {
			return nil
		}

		if request.Header.Get("Content-Encoding") == "deflate" {
			reader, err := compress.NewReader(request.Body, compress.DEFLATE)
			if err != nil {
				return err
			}
			defer reader.Close()

			request.Body = reader
		}

		bodyBytes, err = ioutil.ReadAll(request.Body)

		if err != nil {
			return err
		}

		defer request.Body.Close()

		request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	default:
		return errors.New("request is not a valid type")
	}

	if len(bodyBytes) == 0 {
		return nil
	}

	return json.Unmarshal(bodyBytes, parseTo)
}

func ignoreRedacted() bool {
	zerologLevel := zerolog.GlobalLevel()

	switch zerologLevel {
	case zerolog.DebugLevel:
		return true
	}

	return false
}
