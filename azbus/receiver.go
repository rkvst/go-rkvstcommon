package azbus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/opentracing/opentracing-go"
)

// so we dont have to import the azure repo everywhere
type ReceivedMessage = azservicebus.ReceivedMessage

type Handler interface {
	Handle(context.Context, *ReceivedMessage) error
}

const (
	// RenewalTime is the how often we want to renew the message PEEK lock
	//
	// Inspection of the topics and subscription shows that the PeekLock timeout is one minute.
	//
	// This clarifies the peeklock duration as 60s: https://github.com/MicrosoftDocs/azure-docs/issues/106047
	//
	// "The default lock duration is indeed 1 minute, we will get this updated in our documentation."
	// "As for your question about RenewLock, it's best to set the lock duration to something higher than your normal"
	// "processing time, so you don't have to call the RenewLock. Note that the maximum value is 5 minutes, so you will"
	// "need to call RenewLock if you want to have this longer. Also note that having a longer lock duration then needed"
	// "has some implications as well, f.e. when your client stops working, the message will only become available again"
	// "after the lock duration has passed."
	//
	// An analysis of elapsed times when processing msgs shows no message takes longer than 10s to process during our
	// normal test suites.
	//
	// Set to 50 seconds, well within the 60 seconds peek lock timeout
	RenewalTime = 50 * time.Second
)

// Settings for Receivers:
//
//     NumberOfReceivedMessages  int
//
//         The number of messages fetched simultaneously from azure servicebus.
//         Currently this figure cannot be high (suggest <6) as the peeklock
//         timeout for all msgs starts as soon as fetched. This means that
//         if the processing of a previous msg fetched takes a long time
//         because of send retries then following messages start with some of
//         their peeklock time used up. Currently we have 60s to process all the
//         messages. This can be fixed by setting RenewMessageLock to true.
//         Some services start up multiple handlers and this setting should be 1
//         in this case.
//
//     RenewMessageLock bool
//
//         If true the peeklocktimeout is restarted after 50s. This is currently
//         only true for the simplehasher services as they are anticipated to
//         exceed 50s to process data. Should be safe to be true by default but
//         a policy decision to not set to true by default is deferred until some
//         experience is obtained. Testing with true everywhere revealed no
//         problems.
//         Alternatively we could increase the TTL of the peeklock - the absolute
//         hard limit is 300s.
//
//     Both of these parameters are controlled by helm chart settings.

// ReceiverConfig configuration for an azure servicebus queue
type ReceiverConfig struct {
	ConnectionString string

	// Name is the name of the queue or topic
	TopicOrQueueName string

	// Subscriptioon is the name of the topic subscription.
	// If blank then messages are received from a Queue.
	SubscriptionName string

	// See azbus/receiver.go
	NumberOfReceivedMessages int
	RenewMessageLock         bool

	// If a deadletter receiver then this is true
	Deadletter bool
}

// Receiver to receive messages on  a queue
type Receiver struct {
	azClient AZClient

	Cfg ReceiverConfig

	log      Logger
	mtx      sync.Mutex
	receiver *azservicebus.Receiver
	options  *azservicebus.ReceiverOptions
}

func NewReceiver(log Logger, cfg ReceiverConfig) *Receiver {
	var options *azservicebus.ReceiverOptions
	if cfg.Deadletter {
		options = &azservicebus.ReceiverOptions{
			ReceiveMode: azservicebus.ReceiveModePeekLock,
			SubQueue:    azservicebus.SubQueueDeadLetter,
		}
	}

	r := &Receiver{
		Cfg:      cfg,
		azClient: NewAZClient(cfg.ConnectionString),
		options:  options,
	}
	r.log = log.WithIndex("receiver", r.String())
	return r
}

func (r *Receiver) GetAZClient() AZClient {
	return r.azClient
}

// String - returns string representation of receiver.
func (r *Receiver) String() string {
	// No log function calls in this method please.
	if r.Cfg.SubscriptionName != "" {
		if r.Cfg.Deadletter {
			return fmt.Sprintf("%s.%s.deadletter", r.Cfg.TopicOrQueueName, r.Cfg.SubscriptionName)
		}
		return fmt.Sprintf("%s.%s", r.Cfg.TopicOrQueueName, r.Cfg.SubscriptionName)
	}
	if r.Cfg.Deadletter {
		return fmt.Sprintf("%s.deadletter", r.Cfg.TopicOrQueueName)
	}
	return fmt.Sprintf("%s", r.Cfg.TopicOrQueueName)
}

// elapsed emits 2 log messages detailing how long processing took.
// TODO: emit the processing time as a prometheus metric.
func (r *Receiver) elapsed(ctx context.Context, count int, total int, maxDuration time.Duration, msg *ReceivedMessage, handler Handler) error {
	now := time.Now()
	ctx = ContextFromReceivedMessage(ctx, msg)
	log := r.log.FromContext(ctx)
	defer log.Close()

	log.Debugf("Processing message %d of %d", count, total)
	err := handler.Handle(ctx, msg)
	duration := time.Since(now)
	log.Debugf("Processing message %d took %s", count, duration)
	// This is safe because maxDuration is only defined if RenewMessageLock is false.
	if !r.Cfg.RenewMessageLock && duration >= maxDuration {
		log.Infof("WARNING: processing msg %d duration %v took more than %v seconds", count, duration, maxDuration)
		log.Infof("WARNING: please either enable SERVICEBUS_RENEW_LOCK or reduce SERVICEBUS_INCOMING_MESSAGES")
		log.Infof("WARNING: both can be found in the helm chart for each service.")
	}
	if err != nil {
		if errors.Is(err, ErrPeekLockTimeout) {
			log.Infof("WARNING: processing msg %d duration %s returned error: %v", count, duration, err)
			log.Infof("WARNING: please either enable SERVICEBUS_RENEW_LOCK or reduce SERVICEBUS_INCOMING_MESSAGES")
			log.Infof("WARNING: both can be found in the helm chart for each service.")
		}
	}
	return err

}

// RenewMessageLock renews the given messages peek lock, so it doesn't lose the lock and get re-added to the message queue.
//
// Stop the message lock renewal by cancelling the passed in context
func (r *Receiver) receiverRenewMessageLock(ctx context.Context, count int, msg *ReceivedMessage) {
	var err error

	ticker := time.NewTicker(RenewalTime)

	var counter int
	r.log.Debugf("RenewMessageLock %d started", count)
	for {
		select {
		case <-ctx.Done():
			r.log.Debugf("RenewMessageLock %d stopped after %d executions", count, counter)
			ticker.Stop()
			return
		case t := <-ticker.C:
			counter++
			r.log.Debugf("RenewMessageLock %d (%d)", count, counter)
			err = r.receiver.RenewMessageLock(ctx, msg, nil)
			// if we cannot renew the message, we can't do much but log it
			//
			// worse case scenario, we lose the message peek lock and it gets put back on the message queue and is
			// received again.
			if err != nil {
				azerr := fmt.Errorf("RenewMessageLock %d: failed to renew message lock at %v: %w", count, t, NewAzbusError(err))
				r.log.Infof("%s", azerr)
			}
		}
	}
}

func (r *Receiver) ReceiveMessages(handler Handler) error {

	ctx := context.Background()

	err := r.Open()
	if err != nil {
		azerr := fmt.Errorf("%s: ReceiveMessage failure: %w", r, NewAzbusError(err))
		r.log.Infof("%s", azerr)
		return azerr
	}
	r.log.Debugf(
		"NumberOfReceivedMessages %d, RenewMessageLock: %v",
		r.Cfg.NumberOfReceivedMessages,
		r.Cfg.RenewMessageLock,
	)

	for {
		var err error
		var messages []*ReceivedMessage
		messages, err = r.receiver.ReceiveMessages(ctx, r.Cfg.NumberOfReceivedMessages, nil)
		if err != nil {
			azerr := fmt.Errorf("%s: ReceiveMessage failure: %w", r, NewAzbusError(err))
			r.log.Infof("%s", azerr)
			return azerr
		}
		total := len(messages)
		r.log.Debugf("total messages %d", total)

		err = func(fctx context.Context) error {
			var ectx context.Context // we need a cancellation if RenewMessageLock is enabled
			var ecancel context.CancelFunc
			if r.Cfg.RenewMessageLock {
				// start up RenewMessageLock goroutines before processing any
				// messages.
				ectx, ecancel = context.WithCancel(fctx)
				defer ecancel()
				for i := 0; i < total; i++ {
					go r.receiverRenewMessageLock(ectx, i+1, messages[i])
				}
			}
			// See the setTimeout() function for caveats around setting a timeout based on the
			// msg Deadline. It is questionable whether this is necessary. The cancel is currently
			// ignored by the elapsed function.
			var rctx context.Context // we need a timeout if RenewMessageLock is disabled
			var rcancel context.CancelFunc
			var maxDuration time.Duration
			for i := 0; i < total; i++ {
				msg := messages[i]
				if r.Cfg.RenewMessageLock {
					rctx = fctx
				} else {
					rctx, rcancel, maxDuration = setTimeout(fctx, r.log, msg)
					defer rcancel()
				}
				elapsedErr := r.elapsed(rctx, i+1, total, maxDuration, msg, handler)
				if elapsedErr != nil {
					// return here so that no further messages are processed
					// XXXX: check for ErrPeekLockTimeout and only terminate
					//       then?
					return elapsedErr
				}
			}
			return nil
		}(ctx)
		if err != nil {
			return err
		}
	}

}

func (r *Receiver) Open() error {
	var err error

	if r.receiver != nil {
		return nil
	}
	r.log.Debugf("Open")
	client, err := r.azClient.azClient()
	if err != nil {
		return err
	}

	var receiver *azservicebus.Receiver
	if r.Cfg.SubscriptionName != "" {
		receiver, err = client.NewReceiverForSubscription(r.Cfg.TopicOrQueueName, r.Cfg.SubscriptionName, r.options)
	} else {
		receiver, err = client.NewReceiverForQueue(r.Cfg.TopicOrQueueName, r.options)
	}
	if err != nil {
		azerr := fmt.Errorf("%s: failed to open receiver: %w", r, NewAzbusError(err))
		r.log.Infof("%s", azerr)
		return azerr
	}

	r.receiver = receiver
	return nil
}

func (r *Receiver) Close(ctx context.Context) {
	if r != nil {
		r.mtx.Lock()
		defer r.mtx.Unlock()

		if r.receiver != nil {
			err := r.receiver.Close(ctx)
			if err != nil {
				azerr := fmt.Errorf("%s: Error closing receiver: %w", r, NewAzbusError(err))
				r.log.Infof("%s", azerr)
			}
			r.receiver = nil
		}
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
