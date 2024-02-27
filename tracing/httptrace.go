package tracing

import (
	"context"
	"math"
	"net/http"

	"github.com/google/uuid"
	"github.com/libercapital/liber-logger-go"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
)

type HttpTraceTags []httptrace.RoundTripperOption
type HttpTraceConfig StartContextAndSpanConfig[func(req *http.Request) string, HttpTraceTags]

func (h HttpTraceTags) toSpanTag(opts *[]httptrace.RoundTripperOption) {
	*opts = append(*opts, h...)
}

func HttpTrace(
	httpClient *http.Client,
	traceConfig HttpTraceConfig,
) *http.Client {
	opts := []httptrace.RoundTripperOption{
		httptrace.RTWithServiceName(tracingParams.serviceName),
		httptrace.RTWithSpanNamer(func(req *http.Request) string { return traceConfig.OperationName }),
		httptrace.RTWithResourceNamer(traceConfig.ResourceName),
		httptrace.WithBefore(func(r *http.Request, span ddtrace.Span) {
			logFields := map[string]interface{}{
				"dd.span_id":  span.Context().SpanID(),
				"dd.trace_id": span.Context().TraceID(),
				"log_id":      uuid.NewString(),
			}

			r = r.WithContext(context.WithValue(r.Context(), liberlogger.LogFieldsKey{}, logFields))
		}),
	}

	if !math.IsNaN(traceConfig.AnalyticsRate) {
		opts = append(opts, httptrace.RTWithAnalyticsRate(traceConfig.AnalyticsRate))
	}

	traceConfig.Tags.toSpanTag(&opts)

	return httptrace.WrapClient(httpClient, opts...)
}
