package tracing

import (
	"context"
	"fmt"

	opentracing "github.com/opentracing/opentracing-go"
	otlog "github.com/opentracing/opentracing-go/log"
)

// Spanner is an interface to the underlying opentracing span interfaces.
type Spanner interface {
	InjectContext(context.Context, Logger) (map[string]string, Logger, error)
	Close()
}

type NewSpanFunc func(context.Context, Logger, string, ...map[string]any) (Spanner, context.Context, Logger)

type spanning struct {
	span Span
}

func (sp *spanning) Close() {
	if sp.span != nil {
		sp.span.Finish()
		sp.span = nil
	}
}

// InjectContext extracts the standard b3 fields from the span context if any exist.
func (sp *spanning) InjectContext(ctx context.Context, log Logger) (map[string]string, Logger, error) {
	if sp.span == nil {
		// We may be the NullSpanner
		log.Debugf("InjectContext: null spanner")
		return nil, log, nil
	}
	carrier := opentracing.TextMapCarrier{}
	err := opentracing.GlobalTracer().Inject(sp.span.Context(), opentracing.TextMap, carrier)
	if err != nil {
		log.Infof("Unable to inject span context: %v", err)
		return nil, log, fmt.Errorf("Unable to inject span context: %v", err)
	}
	log.Debugf("InjectContext: carrier %v", carrier)
	traceID, found := carrier[TraceID]
	if found && traceID != "" {
		log = log.WithIndex(TraceID, traceID)
	} else {
		log.Debugf("InjectContext: %s not found", TraceID)
	}
	log.Debugf("InjectContext: carrier map %v", carrier)
	return carrier, log, nil
}

// NullSpan is a dummy Spanner for when tracing is disabled
func NullSpan(
	ctx context.Context,
	log Logger,
	_ string,
	_ ...map[string]any,
) (Spanner, context.Context, Logger) {
	return &spanning{}, ctx, log
}

// NewSpan interrogates the context for the presence of a span and returns aan opaque handler which
// has a Close() method suitable for a defer command.
//
// attrs is variadic but should only have one or two members. The first member denotes tags that
// label the span. The second member is map of acceptable x-b3 and related fields that must be
// inserted into the span carrier.
func NewSpan(
	ctx context.Context,
	log Logger,
	label string,
	attrs ...map[string]any,
) (Spanner, context.Context, Logger) {

	var tags map[string]any
	var carrierAttrs map[string]any
	if len(attrs) > 0 {
		tags = attrs[0]
	}
	if len(attrs) > 1 {
		carrierAttrs = attrs[1]
	}
	var opts = []opentracing.StartSpanOption{}
	carrier := opentracing.TextMapCarrier{}
	// The attributes map is passed into the open tracing constructor which
	// extracts any bits it is interested in to use to setup the spans etc.
	// It will ignore anything it doesn't care about. So the filtering of the map
	// is done for us and we don't need to pre-filter it.
	if carrierAttrs != nil {
		for k, v := range carrierAttrs {
			// Tracing properties will be strings
			value, ok := v.(string)
			if ok {
				log.Debugf("NewSpan: carrier: %s, %s", k, value)
				carrier.Set(k, value)
			}
		}
		ctx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, carrier)
		if err != nil {
			log.Infof("NewSpan: Unable to extract span context: %v", err)
		} else {
			opts = append(opts, opentracing.ChildOf(ctx))
		}
	}

	span := opentracing.StartSpan(label, opts...)
	if tags != nil {
		logFields := make([]otlog.Field, 0, len(tags))
		for k, v := range tags {
			// Tracing fields will be strings
			value, ok := v.(string)
			if ok {
				log.Debugf("NewSpan: logField: %s, %s", k, value)
				logFields = append(logFields, otlog.String(k, value))
			}
		}
		log.Debugf("NewSpan: logFields: %v", logFields)
		span.LogFields(logFields...)
	}

	log.Debugf("NewSpan: carrier %v", carrier)
	traceID, found := carrier[TraceID]
	if found && traceID != "" {
		log = log.WithIndex(TraceID, traceID)
	}
	log.Debugf("NewSpan: x-b3-traceID %s", traceID)
	ctx = opentracing.ContextWithSpan(ctx, span)

	return &spanning{span: span}, ctx, log
}
