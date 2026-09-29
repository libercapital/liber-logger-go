package liberlogger

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	_ http.Flusher  = (*LogResponseWriter)(nil)
	_ http.Hijacker = (*LogResponseWriter)(nil)
	_ interface {
		Unwrap() http.ResponseWriter
	} = (*LogResponseWriter)(nil)
)

type echoMiddlewareCase struct {
	name       string
	middleware echo.MiddlewareFunc
}

func echoMiddlewares(redactKeys []string, maskKeys []string, routesIgnore []string) []echoMiddlewareCase {
	return []echoMiddlewareCase{
		{name: "EchoV4", middleware: EchoV4(routesIgnore)},
		{name: "EchoV4Redacted", middleware: EchoV4Redacted(redactKeys, maskKeys, routesIgnore)},
	}
}

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

func parseLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var lines []map[string]any

	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid log line %q: %v", line, err)
		}

		lines = append(lines, entry)
	}

	return lines
}

type serveResult struct {
	recorder     *httptest.ResponseRecorder
	logs         []map[string]any
	echoWarnings string
}

func serve(t *testing.T, middleware echo.MiddlewareFunc, method string, path string, body io.Reader, handler echo.HandlerFunc) serveResult {
	t.Helper()

	logs := captureLogs(t)
	echoWarnings := &bytes.Buffer{}

	e := echo.New()
	e.Logger.SetOutput(echoWarnings)
	e.Use(middleware)
	e.Add(method, path, handler)

	req := httptest.NewRequest(method, path, body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	return serveResult{recorder: rec, logs: parseLogLines(t, logs), echoWarnings: echoWarnings.String()}
}

func requireTwoLogs(t *testing.T, result serveResult) (map[string]any, map[string]any) {
	t.Helper()

	if len(result.logs) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %v", len(result.logs), result.logs)
	}

	return result.logs[0], result.logs[1]
}

func TestEchoV4_LogsRequestAndResponse(t *testing.T) {
	for _, mw := range echoMiddlewares(DefaultKeys, DefaultKeysToMask, nil) {
		t.Run(mw.name, func(t *testing.T) {
			result := serve(t, mw.middleware, http.MethodGet, "/ok", nil, func(c echo.Context) error {
				return c.JSON(http.StatusOK, map[string]any{"a": 1})
			})

			request, response := requireTwoLogs(t, result)

			if request["message"] != "HTTP Server GET /ok" {
				t.Errorf("unexpected request message: %v", request["message"])
			}

			if response["message"] != "HTTP Server GET 200 /ok" {
				t.Errorf("unexpected response message: %v", response["message"])
			}

			body, _ := response["body"].(map[string]any)
			if body["a"] != float64(1) {
				t.Errorf("unexpected response body: %v", response["body"])
			}

			extra, _ := response["extra"].(map[string]any)
			if extra["status_code"] != float64(http.StatusOK) || extra["url"] != "/ok" || extra["method"] != http.MethodGet {
				t.Errorf("unexpected response extra: %v", extra)
			}
		})
	}
}

func TestEchoV4_LogsHTTPErrorStatus(t *testing.T) {
	for _, mw := range echoMiddlewares(DefaultKeys, DefaultKeysToMask, nil) {
		t.Run(mw.name, func(t *testing.T) {
			result := serve(t, mw.middleware, http.MethodGet, "/missing", nil, func(c echo.Context) error {
				return echo.NewHTTPError(http.StatusNotFound, "not here")
			})

			_, response := requireTwoLogs(t, result)

			if response["message"] != "HTTP Server GET 404 /missing" {
				t.Errorf("unexpected response message: %v", response["message"])
			}

			extra, _ := response["extra"].(map[string]any)
			if extra["status_code"] != float64(http.StatusNotFound) {
				t.Errorf("unexpected status_code: %v", extra["status_code"])
			}

			body, _ := response["body"].(map[string]any)
			if body["message"] != "not here" {
				t.Errorf("unexpected response body: %v", response["body"])
			}

			if result.recorder.Code != http.StatusNotFound {
				t.Errorf("unexpected client status: %d", result.recorder.Code)
			}

			if got := strings.Count(result.recorder.Body.String(), "not here"); got != 1 {
				t.Errorf("expected error body written once, got %d: %q", got, result.recorder.Body.String())
			}

			if strings.Contains(result.echoWarnings, "response already committed") {
				t.Errorf("error handler ran twice: %s", result.echoWarnings)
			}
		})
	}
}

func TestEchoV4_LogsGenericErrorAs500(t *testing.T) {
	for _, mw := range echoMiddlewares(DefaultKeys, DefaultKeysToMask, nil) {
		t.Run(mw.name, func(t *testing.T) {
			result := serve(t, mw.middleware, http.MethodGet, "/boom", nil, func(c echo.Context) error {
				return errors.New("boom")
			})

			_, response := requireTwoLogs(t, result)

			if response["message"] != "HTTP Server GET 500 /boom" {
				t.Errorf("unexpected response message: %v", response["message"])
			}

			if result.recorder.Code != http.StatusInternalServerError {
				t.Errorf("unexpected client status: %d", result.recorder.Code)
			}
		})
	}
}

func TestEchoV4Redacted_RedactsResponse(t *testing.T) {
	result := serve(t, EchoV4Redacted([]string{"access_token", "Authorization"}, []string{"cpf"}, nil), http.MethodGet, "/token", nil, func(c echo.Context) error {
		c.Response().Header().Set("Authorization", "Bearer secret")
		return c.JSON(http.StatusOK, map[string]any{"access_token": "secret", "cpf": "12345678900"})
	})

	_, response := requireTwoLogs(t, result)

	body, _ := response["body"].(map[string]any)
	if body["access_token"] != REDACTED {
		t.Errorf("access_token not redacted: %v", body["access_token"])
	}

	if body["cpf"] != "1234****900" {
		t.Errorf("cpf not masked: %v", body["cpf"])
	}

	headers, _ := response["headers"].(map[string]any)
	if headers["Authorization"] != REDACTED {
		t.Errorf("Authorization header not redacted: %v", headers["Authorization"])
	}
}

func TestEchoV4_LogsNonJSONResponseAsText(t *testing.T) {
	for _, mw := range echoMiddlewares(DefaultKeys, DefaultKeysToMask, nil) {
		t.Run(mw.name, func(t *testing.T) {
			result := serve(t, mw.middleware, http.MethodGet, "/html", nil, func(c echo.Context) error {
				return c.HTML(http.StatusOK, "<html>ok</html>")
			})

			_, response := requireTwoLogs(t, result)

			body, _ := response["body"].(map[string]any)
			if body["plain/text-type"] != "<html>ok</html>" {
				t.Errorf("unexpected response body: %v", response["body"])
			}
		})
	}
}

func TestEchoV4_IgnoredRouteDoesNotLog(t *testing.T) {
	for _, mw := range echoMiddlewares(DefaultKeys, DefaultKeysToMask, []string{"/health"}) {
		t.Run(mw.name, func(t *testing.T) {
			result := serve(t, mw.middleware, http.MethodGet, "/health", nil, func(c echo.Context) error {
				return c.String(http.StatusOK, "UP")
			})

			if len(result.logs) != 0 {
				t.Errorf("expected no logs, got %v", result.logs)
			}

			if result.recorder.Code != http.StatusOK || result.recorder.Body.String() != "UP" {
				t.Errorf("unexpected response: %d %q", result.recorder.Code, result.recorder.Body.String())
			}
		})
	}
}

func TestEchoV4_KeepsRequestBodyForHandler(t *testing.T) {
	for _, mw := range echoMiddlewares(DefaultKeys, DefaultKeysToMask, nil) {
		t.Run(mw.name, func(t *testing.T) {
			var received map[string]any

			result := serve(t, mw.middleware, http.MethodPost, "/items", strings.NewReader(`{"name":"item"}`), func(c echo.Context) error {
				if err := c.Bind(&received); err != nil {
					return err
				}
				return c.JSON(http.StatusCreated, received)
			})

			request, response := requireTwoLogs(t, result)

			if received["name"] != "item" {
				t.Errorf("handler did not receive request body: %v", received)
			}

			requestBody, _ := request["body"].(map[string]any)
			if requestBody["name"] != "item" {
				t.Errorf("request body not logged: %v", request["body"])
			}

			if response["message"] != "HTTP Server POST 201 /items" {
				t.Errorf("unexpected response message: %v", response["message"])
			}
		})
	}
}

func TestEchoV4_SupportsFlush(t *testing.T) {
	for _, mw := range echoMiddlewares(DefaultKeys, DefaultKeysToMask, nil) {
		t.Run(mw.name, func(t *testing.T) {
			result := serve(t, mw.middleware, http.MethodGet, "/stream", nil, func(c echo.Context) error {
				c.Response().WriteHeader(http.StatusOK)
				if _, err := c.Response().Write([]byte("chunk")); err != nil {
					return err
				}
				c.Response().Flush()
				return nil
			})

			_, response := requireTwoLogs(t, result)

			if !result.recorder.Flushed {
				t.Error("response was not flushed")
			}

			if response["message"] != "HTTP Server GET 200 /stream" {
				t.Errorf("unexpected response message: %v", response["message"])
			}
		})
	}
}

func TestEchoV4_LogsResponseWhenRequestBodyIsInvalid(t *testing.T) {
	for _, mw := range echoMiddlewares(DefaultKeys, DefaultKeysToMask, nil) {
		t.Run(mw.name, func(t *testing.T) {
			result := serve(t, mw.middleware, http.MethodPost, "/items", strings.NewReader(`{invalid`), func(c echo.Context) error {
				return c.String(http.StatusBadRequest, "invalid body")
			})

			request, response := requireTwoLogs(t, result)

			if request["level"] != "error" || request["message"] != "HTTP Server | Error when parse in liberlogger POST /items" {
				t.Errorf("unexpected request log: %v", request)
			}

			if response["level"] != "info" || response["message"] != "HTTP Server POST 400 /items" {
				t.Errorf("unexpected response log: %v", response)
			}
		})
	}
}
