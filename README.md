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

    e.Use(liberlogger.EchoV4Redacted(liberlogger.DefaultKeys, []string{}))

    //aditional keys
    redactKeys := liberlogger.DefaultKeys

    copy(redactKeys, []string{"reference_uuid", "document_number"})

    e.Use(liberlogger.EchoV4Redacted(redactKeys, []string{"/health"}))
}
```

</details>

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
