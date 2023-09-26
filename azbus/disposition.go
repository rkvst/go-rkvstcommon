package azbus

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/opentracing/opentracing-go"
)

type Disposition int

const (
	Unknown Disposition = iota
	Abandon
	Complete
	Deadletter
	Reschedule
)

func (d Disposition) String() string {
	switch d {
	case Abandon:
		return "Abandon"
	case Complete:
		return "Complete"
	case Deadletter:
		return "Deadletter"
	case Reschedule:
		return "Reschedule"
	default:
		return fmt.Sprintf("Unknown%d", int(d))
	}
}

func StringToDisposition(s string) Disposition {
	switch s {
	case "Abandon":
		return Abandon
	case "Complete":
		return Complete
	case "Deadletter":
		return Deadletter
	case "Reschedule":
		return Reschedule
	default:
		return Unknown
	}
}

func Dispose(ctx context.Context, d Disposition, r *Receiver, err error, msg *ReceivedMessage) error {
	switch d {
	case Abandon:
		return r.Abandon(ctx, err, msg)
	case Complete:
		return r.Complete(ctx, msg)
	case Deadletter:
		return r.DeadLetter(ctx, err, msg)
	case Reschedule:
		return r.Reschedule(ctx, err, msg)
	default:
		return errors.New("unknown disposition")
	}
}

// NB: ALL disposition methods return nil so they can be used in return statements

// Abandon abandons message. This function is not used but is present for consistency.
func (r *Receiver) Abandon(ctx context.Context, err error, msg *ReceivedMessage) error {
	ctx = context.WithoutCancel(ctx)
	log := r.log.FromContext(ctx)
	defer log.Close()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Message.Abandon")
	defer span.Finish()
	log.Infof("Abandon Message on DeliveryCount %d: %v", msg.DeliveryCount, err)
	err1 := r.receiver.AbandonMessage(ctx, msg, nil)
	if err1 != nil {
		azerr := fmt.Errorf("Abandon Message failure: %w", NewAzbusError(err1))
		log.Infof("%s", azerr)
	}
	return nil
}

// Reschedule handles when a message should be deferred at a later time. There are a
// number of ways of doing this but it turns out that simply not doing anything causes
// azservicebus to resubmit the message 1 minute later. We keep the function signature with
// unused arguments for consistency and in case we need to implement more sophisticated
// algorithms in future.
func (r *Receiver) Reschedule(ctx context.Context, err error, msg *ReceivedMessage) error {
	ctx = context.WithoutCancel(ctx)
	log := r.log.FromContext(ctx)
	defer log.Close()

	span, _ := opentracing.StartSpanFromContext(ctx, "Message.Reschedule")
	defer span.Finish()
	log.Infof("Reschedule Message on DeliveryCount %d: %v", msg.DeliveryCount, err)
	return nil
}

// DeadLetter explicitly deadletters a message.
func (r *Receiver) DeadLetter(ctx context.Context, err error, msg *ReceivedMessage) error {
	ctx = context.WithoutCancel(ctx)
	log := r.log.FromContext(ctx)
	defer log.Close()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Message.DeadLetter")
	defer span.Finish()
	log.Infof("DeadLetter Message: %v", err)
	options := azservicebus.DeadLetterOptions{
		Reason: to.Ptr(err.Error()),
	}
	err1 := r.receiver.DeadLetterMessage(ctx, msg, &options)
	if err1 != nil {
		azerr := fmt.Errorf("DeadLetter Message failure: %w", NewAzbusError(err1))
		log.Infof("%s", azerr)
	}
	return nil
}

func (r *Receiver) Complete(ctx context.Context, msg *ReceivedMessage) error {
	ctx = context.WithoutCancel(ctx)
	log := r.log.FromContext(ctx)
	defer log.Close()

	span, _ := opentracing.StartSpanFromContext(ctx, "Message.Complete")
	defer span.Finish()

	log.Infof("Complete Message")

	err := r.receiver.CompleteMessage(ctx, msg, nil)
	if err != nil {
		// If the completion fails then the message will get rescheduled, but it's effect will
		// have been made, so we could get duplication issues.
		azerr := fmt.Errorf("Complete: failed to settle message: %w", NewAzbusError(err))
		log.Infof("%s", azerr)
	}
	return nil
}
