package azbus

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/datatrails/go-datatrails-common/logger"
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

type Disposer interface {
	Dispose(ctx context.Context, d Disposition, err error, msg *ReceivedMessage)
}

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

func (r *Receiver) dispose(ctx context.Context, d Disposition, err error, msg *ReceivedMessage) {
	switch {
	case d == DeadletterDisposition:
		r.deadLetter(ctx, err, msg)
		return
	case d == AbandonDisposition:
		r.abandon(ctx, err, msg)
		return
	case d == RescheduleDisposition:
		r.reschedule(ctx, err, msg)
		return
	case d == CompleteDisposition:
		r.complete(ctx, err, msg)
		return
	}
}

func (r *BatchReceiver) Dispose(ctx context.Context, d Disposition, err error, msg *ReceivedMessage) {
	switch {
	case d == DeadletterDisposition:
		r.deadLetter(ctx, err, msg)
		return
	case d == AbandonDisposition:
		r.abandon(ctx, err, msg)
		return
	case d == RescheduleDisposition:
		r.reschedule(ctx, err, msg)
		return
	case d == CompleteDisposition:
		r.complete(ctx, err, msg)
		return
	}
}

// Abandon abandons message. This function is not used but is present for consistency.
func abandon(ctx context.Context, log logger.Logger, label string, sp NewSpanFunc, r *azservicebus.Receiver, err error, msg *ReceivedMessage) {
	ctx = context.WithoutCancel(ctx)

	var span Spanner
	span, ctx, log = sp(ctx, log, "Message.Abandon")
	defer span.Close()

	log.Infof("Abandon Message on DeliveryCount %d: %v", msg.DeliveryCount, err)
	err1 := r.AbandonMessage(ctx, msg, nil)
	if err1 != nil {
		azerr := fmt.Errorf("Abandon Message failure: %w", NewAzbusError(err1))
		log.Infof("%s", azerr)
	}
}

// DeadLetter explicitly deadletters a message.
func deadLetter(ctx context.Context, log logger.Logger, label string, sp NewSpanFunc, r *azservicebus.Receiver, err error, msg *ReceivedMessage) {
	ctx = context.WithoutCancel(ctx)

	var span Spanner
	span, ctx, log = sp(ctx, log, "Message.Deadletter")
	defer span.Close()

	log.Infof("DeadLetter Message: %v", err)
	options := azservicebus.DeadLetterOptions{
		Reason: to.Ptr(strings.ToValidUTF8(err.Error(), "!!!")),
	}
	err1 := r.DeadLetterMessage(ctx, msg, &options)
	if err1 != nil {
		azerr := fmt.Errorf("DeadLetter Message failure: %w", NewAzbusError(err1))
		log.Infof("%s", azerr)
	}
}

func complete(ctx context.Context, log logger.Logger, label string, sp NewSpanFunc, r *azservicebus.Receiver, err error, msg *ReceivedMessage) {
	ctx = context.WithoutCancel(ctx)

	var span Spanner
	span, ctx, log = sp(ctx, log, "Message.Complete")
	defer span.Close()

	if err != nil {
		log.Infof("Complete Message %v", err)
	} else {
		log.Debugf("Complete Message")
	}

	err1 := r.CompleteMessage(ctx, msg, nil)
	if err1 != nil {
		// If the completion fails then the message will get rescheduled, but it's effect will
		// have been made, so we could get duplication issues.
		azerr := fmt.Errorf("Complete: failed to settle message: %w", NewAzbusError(err1))
		log.Infof("%s", azerr)
	}
}

// Reschedule handles when a message should be deferred at a later time. There are a
// number of ways of doing this but it turns out that simply not doing anything causes
// azservicebus to resubmit the message 1 minute later. We keep the function signature with
// unused arguments for consistency and in case we need to implement more sophisticated
// algorithms in future.
func reschedule(ctx context.Context, log logger.Logger, label string, sp NewSpanFunc, r *azservicebus.Receiver, err error, msg *ReceivedMessage) {
	ctx = context.WithoutCancel(ctx)

	var span Spanner
	span, _, log = sp(ctx, log, "Message.Reschedule")
	defer span.Close()

	log.Infof("Reschedule Message on DeliveryCount %d: %v", msg.DeliveryCount, err)
}

// Vanilla receiver.
func (r *Receiver) abandon(ctx context.Context, err error, msg *ReceivedMessage) {
	abandon(ctx, r.log, r.String(), r.spanner, r.receiver, err, msg)
}

func (r *Receiver) reschedule(ctx context.Context, err error, msg *ReceivedMessage) {
	reschedule(ctx, r.log, r.String(), r.spanner, r.receiver, err, msg)
}

func (r *Receiver) deadLetter(ctx context.Context, err error, msg *ReceivedMessage) {
	deadLetter(ctx, r.log, r.String(), r.spanner, r.receiver, err, msg)
}

func (r *Receiver) complete(ctx context.Context, err error, msg *ReceivedMessage) {

	complete(ctx, r.log, r.String(), r.spanner, r.receiver, err, msg)
}

// Batch receiver.
func (r *BatchReceiver) abandon(ctx context.Context, err error, msg *ReceivedMessage) {
	abandon(ctx, r.log, r.String(), r.spanner, r.Receiver, err, msg)
}

func (r *BatchReceiver) reschedule(ctx context.Context, err error, msg *ReceivedMessage) {
	reschedule(ctx, r.log, r.String(), r.spanner, r.Receiver, err, msg)
}

func (r *BatchReceiver) deadLetter(ctx context.Context, err error, msg *ReceivedMessage) {
	deadLetter(ctx, r.log, r.String(), r.spanner, r.Receiver, err, msg)
}

func (r *BatchReceiver) complete(ctx context.Context, err error, msg *ReceivedMessage) {
	complete(ctx, r.log, r.String(), r.spanner, r.Receiver, err, msg)
}
