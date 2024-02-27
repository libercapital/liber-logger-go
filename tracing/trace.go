package tracing

import "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

var tracingParams = struct {
	serviceName string
}{}

func StartTrace(serviceName string, envLevel string, tracerOptions ...tracer.StartOption) {
	tracingParams.serviceName = serviceName

	opts := []tracer.StartOption{
		tracer.WithService(serviceName),
		tracer.WithEnv(envLevel),
	}

	if len(tracerOptions) > 0 {
		opts = append(opts, tracerOptions...)
	}

	tracer.Start(opts...)
}

func StopTrace() {
	tracer.Stop()
}
