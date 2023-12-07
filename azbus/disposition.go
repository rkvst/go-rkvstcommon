package azbus

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/datatrails/go-datatrails-common/tracing"
)

// Disposition describes the eventual demise of the message after processing by the client.
// Upstream is notified whether the message can be deleted, deadlettered or will be reprocessed later.
type Disposition int

const (
	DeadletterDisposition Disposition = iota
	AbandonDisposition
	RescheduleDisposition
	CompleteDisposition
)

func (d Disposition) String() string {
	switch {
	case d == DeadletterDisposition:
		return "DeadLetter"
	case d == AbandonDisposition:
		return "Abandon"
	case d == RescheduleDisposition:
		return "Reschedule"
	case d == CompleteDisposition:
		return "Complete"
	}
	return fmt.Sprintf("Unknown%d", d)
}

func (r *Receiver) Dispose(ctx context.Context, d Disposition, err error, msg *ReceivedMessage) error {
	switch {
	case d == DeadletterDisposition:
		return r.DeadLetter(ctx, err, msg)
	case d == AbandonDisposition:
		return r.Abandon(ctx, err, msg)
	case d == RescheduleDisposition:
		return r.Reschedule(ctx, err, msg)
	case d == CompleteDisposition:
		return r.parallelComplete(ctx, err, msg)
	}
	return fmt.Errorf("Unknown%d", d)
}

// NB: ALL disposition methods return nil so they can be used in return statements

// Abandon abandons message. This function is not used but is present for consistency.
func (r *Receiver) Abandon(ctx context.Context, err error, msg *ReceivedMessage) error {
	ctx = context.WithoutCancel(ctx)
	log := r.log.FromContext(ctx)
	defer log.Close()

	span, ctx := tracing.StartSpanFromContext(ctx, "Message.Abandon")
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

	span, _ := tracing.StartSpanFromContext(ctx, "Message.Reschedule")
	defer span.Finish()
	log.Infof("Reschedule Message on DeliveryCount %d: %v", msg.DeliveryCount, err)
	return nil
}

// DeadLetter explicitly deadletters a message.
func (r *Receiver) DeadLetter(ctx context.Context, err error, msg *ReceivedMessage) error {
	ctx = context.WithoutCancel(ctx)
	log := r.log.FromContext(ctx)
	defer log.Close()

	span, ctx := tracing.StartSpanFromContext(ctx, "Message.DeadLetter")
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

	span, _ := tracing.StartSpanFromContext(ctx, "Message.Complete")
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

// like Complete but an error may indicate that this will be tried again. Useful for polling type operations
func (r *Receiver) parallelComplete(ctx context.Context, err error, msg *ReceivedMessage) error {
	ctx = context.WithoutCancel(ctx)
	log := r.log.FromContext(ctx)
	defer log.Close()

	span, _ := tracing.StartSpanFromContext(ctx, "Message.Complete")
	defer span.Finish()

	if err != nil {
		log.Infof("Complete Message %v", err)
	} else {
		log.Infof("Complete Message")
	}

	err1 := r.receiver.CompleteMessage(ctx, msg, nil)
	if err1 != nil {
		// If the completion fails then the message will get rescheduled, but it's effect will
		// have been made, so we could get duplication issues.
		azerr := fmt.Errorf("Complete: failed to settle message: %w", NewAzbusError(err1))
		log.Infof("%s", azerr)
	}
	return nil
}
