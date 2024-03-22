package azbus

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
)

func (r *Receiver) handleReceivedMessageWithTracingContext(ctx context.Context, message *ReceivedMessage, handler Handler) (Disposition, context.Context, error) {
	log := r.log.FromContext(ctx)
	defer log.Close()

	log.Debugf("ContextFromReceivedMessage(): ApplicationProperties %v", message.ApplicationProperties)
	var opts = []opentracing.StartSpanOption{}
	carrier := opentracing.TextMapCarrier{}
	// This just gets all the message Application Properties into a string map. That map is then passed into the
	// open tracing constructor which extracts any bits it is interested in to use to setup the spans etc.
	// It will ignore anything it doesn't care about. So the filtering of the map is done for us and
	// we don't need to pre-filter it.
	for k, v := range message.ApplicationProperties {
		// XXX: why only string values?
		value, ok := v.(string)
		if ok {
			carrier.Set(k, value)
		}
	}
	spanCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, carrier)
	if err != nil {
		log.Infof("handleReceivedMessageWithTracingContext(): Unable to extract span context: %v", err)
	} else {
		opts = append(opts, opentracing.ChildOf(spanCtx))
	}
	span := opentracing.StartSpan("handle message", opts...)
	defer span.Finish()
	ctx = opentracing.ContextWithSpan(ctx, span)
	return handler.Handle(ctx, message)
}

func (s *Sender) updateSendingMesssageForSpan(ctx context.Context, message *OutMessage, span opentracing.Span) {
	log := s.log.FromContext(ctx)
	defer log.Close()

	carrier := opentracing.TextMapCarrier{}
	err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, carrier)
	if err != nil {
		log.Infof("updateSendingMesssageForSpan(): Unable to inject span context: %v", err)
		return
	}
	for k, v := range carrier {
		OutMessageSetProperty(message, k, v)
	}
	log.Debugf("updateSendingMesssageForSpan(): ApplicationProperties %v", OutMessageProperties(message))
}
