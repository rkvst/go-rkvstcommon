package azbus

import (
	"context"

	"github.com/datatrails/go-datatrails-common/tracing"
	opentracing "github.com/opentracing/opentracing-go"
	otlog "github.com/opentracing/opentracing-go/log"
)

type Spanner interface {
	Open(context.Context, string, ...[2]string) context.Context
	Close()
	Log() Logger
	InjectContext(context.Context) (map[string]string, error)
}

type spanning struct {
	log  Logger
	span tracing.Span
}

func NewSpanning(log Logger) *spanning {
	return &spanning{log: log}
}

func (sp *spanning) Open(ctx context.Context, label string, fields ...[2]string) context.Context {

	//log := sp.log.FromContext(ctx)
	sp.span, ctx = tracing.StartSpanFromContext(ctx, label)

	if len(fields) > 0 {
		stringFields := make([]otlog.Field, len(fields))
		for i := range fields {
			stringFields[i] = otlog.String(fields[i][0], fields[i][1])
		}
		sp.span.LogFields(stringFields...)
	}
	return ctx
}

func (sp *spanning) Close() {
	if sp.span != nil {
		sp.span.Finish()
		sp.span = nil
	}
	if sp.log != nil {
		sp.log.Close()
		sp.log = nil
	}
}

func (sp *spanning) Log() Logger {
	return sp.log
}

func (sp *spanning) InjectContext(ctx context.Context) (map[string]string, error) {
	carrier := opentracing.TextMapCarrier{}
	err := opentracing.GlobalTracer().Inject(sp.span.Context(), opentracing.TextMap, carrier)
	if err != nil {
		sp.log.Infof("updateSendingMesssage(): Unable to inject span context: %v", err)
		return nil, err
	}
	return carrier, nil
}

func createReceivedMessageTracingContext(ctx context.Context, log Logger, message *ReceivedMessage, handler Handler) (context.Context, opentracing.Span) {
	// We don't have the tracing span info on the context yet, that is what this function will add
	// we log using the receiver logger
	log.Debugf("ContextFromReceivedMessage(): ApplicationProperties %v", message.ApplicationProperties)

	var opts = []opentracing.StartSpanOption{}
	carrier := opentracing.TextMapCarrier{}
	// This just gets all the message Application Properties into a string map. That map is then passed into the
	// open tracing constructor which extracts any bits it is interested in to use to setup the spans etc.
	// It will ignore anything it doesn't care about. So the filtering of the map is done for us and
	// we don't need to pre-filter it.
	for k, v := range message.ApplicationProperties {
		// Tracing properties will be strings
		value, ok := v.(string)
		if ok {
			carrier.Set(k, value)
		}
	}
	spanCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, carrier)
	if err != nil {
		log.Infof("CreateReceivedMessageWithTracingContext(): Unable to extract span context: %v", err)
	} else {
		opts = append(opts, opentracing.ChildOf(spanCtx))
	}
	span := opentracing.StartSpan("handle message", opts...)
	ctx = opentracing.ContextWithSpan(ctx, span)
	return ctx, span
}

func handleReceivedMessageWithTracingContext(ctx context.Context, log Logger, message *ReceivedMessage, handler Handler) (Disposition, context.Context, error) {
	ctx, span := createReceivedMessageTracingContext(ctx, log, message, handler)
	defer span.Finish()
	return handler.Handle(ctx, message)
}
