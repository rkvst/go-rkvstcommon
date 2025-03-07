package tracing

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
)

// LogFromContext returns a logger indexed according to the x-b3-traceID key.
func LogFromContext(ctx context.Context, log Logger) Logger {

	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		log.Infof("LogFromContext: span is nil - this should not happen - the context where this happened is missing tracing content - probably a middleware problem")
		return log
	}
	carrier := opentracing.TextMapCarrier{}
	err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, carrier)
	if err != nil {
		log.Infof("LogFromContext: can't inject span: %v", err)
		return log
	}

	traceID, found := carrier[TraceID]
	if !found || traceID == "" {
		log.Debugf("%s not found", TraceID)
		return log
	}
	return log.WithIndex(TraceID, traceID)
}
