package azbus

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
)

func (r *Receiver) handleReceivedMessageWithTracingContext(ctx context.Context, message *ReceivedMessage, handler Handler) (Disposition, context.Context, error) {
	// We don't have the tracing span info on the context yet, that is what this function will add
	// we log using the receiver logger
	r.log.Debugf("ContextFromReceivedMessage(): ApplicationProperties %v", message.ApplicationProperties)

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
		r.log.Infof("handleReceivedMessageWithTracingContext(): Unable to extract span context: %v", err)
	} else {
		opts = append(opts, opentracing.ChildOf(spanCtx))
	}
	span := opentracing.StartSpan("handle message", opts...)
	defer span.Finish()
	ctx = opentracing.ContextWithSpan(ctx, span)
	return handler.Handle(ctx, message)
}

func (r *Receiver) handleReceivedMessagesWithTracingContext(ctx context.Context, messages []*ReceivedMessage, handler BatchHandler) ([]Disposition, context.Context, error) {

	var opts = []opentracing.StartSpanOption{}
	carrier := opentracing.TextMapCarrier{}
	// This just gets all the message Application Properties into a string map. That map is then passed into the
	// open tracing constructor which extracts any bits it is interested in to use to setup the spans etc.
	// It will ignore anything it doesn't care about. So the filtering of the map is done for us and
	// we don't need to pre-filter it.

	// We don't have the tracing span info on the context yet, that is what this function will add
	// we log using the receiver logger
	// only use first message for tracing..... will fix later
	msg := messages[0]
	r.log.Debugf("ContextFromReceivedMessage(): ApplicationProperties %v", msg.ApplicationProperties)

	for k, v := range msg.ApplicationProperties {
		// Tracing properties will be strings
		value, ok := v.(string)
		if ok {
			carrier.Set(k, value)
		}
	}
	spanCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, carrier)
	if err != nil {
		r.log.Infof("handleReceivedMessageWithTracingContext(): Unable to extract span context: %v", err)
	} else {
		opts = append(opts, opentracing.ChildOf(spanCtx))
	}
	span := opentracing.StartSpan("handle message", opts...)
	defer span.Finish()
	ctx = opentracing.ContextWithSpan(ctx, span)
	return handler.Handle(ctx, messages)
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
