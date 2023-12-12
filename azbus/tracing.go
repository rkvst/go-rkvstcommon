package azbus

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
)

func (r *Receiver) handleReceivedMessageWithTracingContext(ctx context.Context, message *ReceivedMessage, handler Handler) (Disposition, context.Context, error) {
	log := r.log.FromContext(ctx)

	log.Debugf("ContextFromReceivedMessage(): ApplicationProperties %v", message.ApplicationProperties)
	var opts = []opentracing.StartSpanOption{}
	carrier := opentracing.TextMapCarrier{}
	for k, v := range message.ApplicationProperties {
		value, ok := v.(string)
		if ok {
			carrier.Set(k, value)
		}
	}
	spanCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, carrier)
	if err != nil {
		log.Infof("HandleReceivedMessageWithTracingContext(): Unable to extract span context: %v", err)
	} else {
		opts = append(opts, opentracing.ChildOf(spanCtx))
	}
	span := opentracing.StartSpan("Handle message", opts...)
	defer span.Finish()
	ctx = opentracing.ContextWithSpan(ctx, span)
	return handler.Handle(ctx, message)
}

func (s *Sender) UpdateSendingMesssageForSpan(ctx context.Context, message *OutMessage, span opentracing.Span) {
	log := s.log.FromContext(ctx)

	carrier := opentracing.TextMapCarrier{}
	err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, carrier)
	if err != nil {
		log.Infof("UpdateSendingMesssageForSpan(): Unable to inject span context: %v", err)
		return
	}
	for k, v := range carrier {
		message.ApplicationProperties[k] = v
	}
	log.Debugf("UpdateSendingMesssageForSpan(): ApplicationProperties %v", message.ApplicationProperties)
}
