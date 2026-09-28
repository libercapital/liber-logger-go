# Welcome to liberlogger 👋

> Centralized logs to yours applications.

## How to use

<br />

```
go get github.com/libercapital/liber-logger-go.git
```

### Basic logs

```golang
package main

import "github.com/libercapital/liber-logger-go.git"

func main() {
    ctx := Context.Background()

    liberlogger.Init(os.Getenv("LOG_LEVEL"))

    liberlogger.Info(ctx).Msg("send a msg with info level")

    liberlogger.Debug(ctx).Msg("send a msg with debug level")

    errorTest := errors.New("error test")
    liberlogger.Error(ctx, errorTest).Msg("send a msg with error level")

    liberlogger.Warn(ctx).Msg("send a msg with warn level")

    errorTest = errors.New("fatal error test")
    liberlogger.Fatal(ctx, errorTest).Msg("send a msg with error level")
}
```

### Echo V4

<details>
    <summary>Unredacted</summary>

```golang
package main

import(
    "github.com/libercapital/liber-logger-go.git"
    "github.com/labstack/echo/v4"
)

func main() {
    liberlogger.Init(os.Getenv("LOG_LEVEL"))

    e := echo.New()

    e.Use(liberlogger.EchoV4([]string{"/health"}))
}
```

</details>

<details>
    <summary>Redacted</summary>

```golang
package main

import(
    "github.com/libercapital/liber-logger-go.git"
    "github.com/labstack/echo/v4"
)

func main() {
    liberlogger.Init(os.Getenv("LOG_LEVEL"))

    e := echo.New()

    e.Use(liberlogger.EchoV4Redacted(liberlogger.DefaultKeys, liberlogger.DefaultKeysToMask, []string{"/health"}))

    //aditional keys
    redactKeys := append(liberlogger.DefaultKeys, "reference_uuid", "document_number")
    maskKeys := append(liberlogger.DefaultKeysToMask, "phone")

    e.Use(liberlogger.EchoV4Redacted(redactKeys, maskKeys, []string{"/health"}))
}
```

</details>

Every request that is not in the ignore list produces two logs:

| Log | Message | Attributes |
|---|---|---|
| Request | `HTTP Server GET /payment-link/v1/checkout/955874a6-...` | `headers`, `body` and `extra` (`url`, `method`) of the request |
| Response | `HTTP Server GET 200 /payment-link/v1/checkout/955874a6-...` | `headers`, `body` and `extra` (`url`, `method`, `status_code`) of the response |

- The response status is the one sent to the client, including errors returned by the handler: the middleware calls `c.Error(err)` before logging and then returns `err`, like echo's `RequestLogger` with `HandleError: true`. A custom `HTTPErrorHandler` must skip responses that are already committed (`c.Response().Committed`), as the default one does.
- Non-JSON response bodies (HTML, plain text) are logged as `{"plain/text-type": "<body>"}`.
- `EchoV4Redacted` applies the redact and mask keys to the response headers and body too.
- Register compression middlewares (e.g. `middleware.Gzip()`) before the logger, so it captures the uncompressed body.
- A panicking handler recovered by an outer `middleware.Recover()` only produces the request log, since the panic unwinds past the logger (same as echo's `RequestLogger`).

<br />

### HTTP Client

<details>
    <summary>Unredacted</summary>

```golang
package main

import (
    "net/http"

    "github.com/libercapital/liber-logger-go.git"
)

func main() {
    liberlogger.Init(os.Getenv("LOG_LEVEL"))

    httpClient := &http.Client{
        Transport: liberlogger.HttpClient{
            Proxied:      http.DefaultTransport,
        },
    }

    httpClient.Get("https://google.com.br")
}
```

</details>

<details>
    <summary>Redacted</summary>

```golang
package main

import (
    "net/http"

    "github.com/libercapital/liber-logger-go.git"
)

func main() {
    liberlogger.Init(os.Getenv("LOG_LEVEL"))

    httpClient := &http.Client{
        Transport: liberlogger.HttpClient{
            Proxied:      http.DefaultTransport,
            RedactedKeys: liberlogger.DefaultKeys,
        },
    }

    httpClient.Get("https://google.com.br")
}
```

</details>

<br />

### Gorilla Mux

<details>
    <summary>Unredacted</summary>

```golang
package main

import (
    "bytes"
    "fmt"
    "net/http"

    "github.com/gorilla/mux"
    "github.com/libercapital/liber-logger-go.git"
)

func main() {
    liberlogger.Init(os.Getenv("LOG_LEVEL"))

    r := mux.NewRouter()

    r.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(rw, "ok")
    })

    r.Use(liberlogger.GorillaMux([]string{"/health"}))

    http.ListenAndServe(":8085", r)
}
```

</details>

<details>
    <summary>Redacted</summary>

```golang
package main

import (
    "bytes"
    "fmt"
    "net/http"

    "github.com/gorilla/mux"
    "github.com/libercapital/liber-logger-go.git"
)

func main() {
    liberlogger.Init(os.Getenv("LOG_LEVEL"))

    r := mux.NewRouter()

    r.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(rw, "ok")
    })

    r.Use(liberlogger.GorillaMuxRedacted(liberlogger.DefaultKeys, []string{"/health"}))

    http.ListenAndServe(":8085", r)
}
```

</details>

---

### Starting Data Dog Span and getting a Context

#### Controller method

```golang
ctx, span := liberlogger.StartContextAndTrace(liberlogger.StartContextAndTraceConfig{
			Ctx: c.Request().Context(),
		})
		defer span.Finish()
```

#### Amqp consumer method

```golang
ctx, span := liberlogger.StartContextAndTrace(liberlogger.StartContextAndTraceConfig{
			ServiceName: "service-name",
			OperationName: "invoice.cmd.creation",
		})
		defer span.Finish()
```
