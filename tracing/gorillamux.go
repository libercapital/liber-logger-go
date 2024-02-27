package tracing

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/libercapital/liber-logger-go"

	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

type GorillaMuxTags []muxtrace.RouterOption
type GorillaMuxConfig StartContextAndSpanConfig[func(router *muxtrace.Router, req *http.Request) string, GorillaMuxTags]

func (g GorillaMuxTags) toSpanTag(opts *[]muxtrace.RouterOption) {
	*opts = append(*opts, g...)
}

func GorillaMuxTrace(traceConfig GorillaMuxConfig) *muxtrace.Router {
	opts := []muxtrace.RouterOption{
		muxtrace.WithServiceName(tracingParams.serviceName),
		muxtrace.WithResourceNamer(traceConfig.ResourceName),
		muxtrace.WithIgnoreRequest(func(req *http.Request) bool {
			return req.URL.Path == "/health"
		}),
	}

	traceConfig.Tags.toSpanTag(&opts)

	r := muxtrace.NewRouter(opts...)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if span, ok := SpanFromContext(ctx); ok {
				logFields := map[string]interface{}{
					"dd.span_id":  span.Context().SpanID(),
					"dd.trace_id": span.Context().TraceID(),
					"log_id":      uuid.NewString(),
				}

				r = r.WithContext(context.WithValue(ctx, liberlogger.LogFieldsKey{}, logFields))
			}

			next.ServeHTTP(w, r)
		})
	})

	return r
}
