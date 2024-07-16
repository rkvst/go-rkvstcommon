package azbus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

var (
	ErrNoHandler = errors.New("no handler defined")
)

// Handler processes a ReceivedMessage.
type Handler interface {
	Handle(context.Context, *ReceivedMessage) (Disposition, context.Context, error)
	Open() error
	Close()
}

// BatchHandler processes ReceivedMessages.
type BatchHandler interface {
	Handle(context.Context, []*ReceivedMessage) ([]Disposition, context.Context, error)
	Open() error
	Close()
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
	RenewMessageLock bool
	RenewMessageTime time.Duration

	// If a deadletter receiver then this is true
	Deadletter bool
}

// Receiver to receive messages on  a queue
type Receiver struct {
	azClient AZClient

	Cfg ReceiverConfig

	log                      Logger
	mtx                      sync.Mutex
	receiver                 *azservicebus.Receiver
	options                  *azservicebus.ReceiverOptions
	handlers                 []Handler
	serialHandler            Handler
	batchHandler             BatchHandler
	numberOfReceivedMessages int // for serial or Batch Handler only
}

type ReceiverOption func(*Receiver)

// WithHandlers
func WithHandlers(h ...Handler) ReceiverOption {
	return func(r *Receiver) {
		r.handlers = append(r.handlers, h...)
	}
}

// WithBatchHandler
func WithBatchHandler(h BatchHandler, n int) ReceiverOption {
	return func(r *Receiver) {
		r.batchHandler = h
		r.numberOfReceivedMessages = n
	}
}

// WithSerialHandler
func WithSerialHandler(h Handler, n int) ReceiverOption {
	return func(r *Receiver) {
		r.serialHandler = h
		r.numberOfReceivedMessages = n
	}
}

// WithRenewalTime takes an optional time to renew the peek lock. This should be comfortably less
// than the peek lock timeout. For example: the default peek lock timeout is 60s and the default
// renewal time is 50s.
//
// Note! Only use this if you know what you're doing and you require custom timeout behaviour. The
// peek lock timeout is specified in terraform configs currently, as it is a property of
// subscriptions or queues.
func WithRenewalTime(t int) ReceiverOption {
	return func(r *Receiver) {
		r.Cfg.RenewMessageTime = time.Duration(t) * time.Second
	}
}

// NewReciver creates a new Receiver that will process a number of messages simultaneously.
// Each handler executes in its own goroutine.
func NewReceiver(log Logger, cfg ReceiverConfig, opts ...ReceiverOption) *Receiver {
	var r Receiver
	return newReceiver(&r, log, cfg, opts...)
}

// function outlining (look it up).
func newReceiver(r *Receiver, log Logger, cfg ReceiverConfig, opts ...ReceiverOption) *Receiver {
	var options *azservicebus.ReceiverOptions
	if cfg.Deadletter {
		options = &azservicebus.ReceiverOptions{
			ReceiveMode: azservicebus.ReceiveModePeekLock,
			SubQueue:    azservicebus.SubQueueDeadLetter,
		}
	}

	r.Cfg = cfg
	r.azClient = NewAZClient(cfg.ConnectionString)
	r.options = options
	r.handlers = []Handler{}
	r.log = log.WithIndex("receiver", r.String())
	for _, opt := range opts {
		opt(r)
	}

	// Set this to a default that corresponds to the az servicebus default peek-lock timeout
	if r.Cfg.RenewMessageTime == 0 {
		r.Cfg.RenewMessageTime = RenewalTime
	}

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

// processMessage disposes of messages and emits 2 log messages detailing how long processing took.
func (r *Receiver) processMessages(ctx context.Context, maxDuration time.Duration, messages []*ReceivedMessage, handler BatchHandler) {
	count := len(messages)
	now := time.Now()
	dispositions, ctx, err := r.handleReceivedMessagesWithTracingContext(ctx, messages, handler)
	for i, disp := range dispositions {
		r.dispose(ctx, disp, err, messages[i])
	}
	duration := time.Since(now)

	// Now we do have a tracing context we can use it for logging
	log := r.log.FromContext(ctx)
	defer log.Close()

	log.Debugf("Processing messages %d took %s", len(messages), duration)

	// This is safe because maxDuration is only defined if RenewMessageLock is false.
	if !r.Cfg.RenewMessageLock && duration >= maxDuration {
		log.Infof("WARNING: processing msg %d duration %v took more than %v seconds", count, duration, maxDuration)
		log.Infof("WARNING: please either enable SERVICEBUS_RENEW_LOCK or reduce SERVICEBUS_INCOMING_MESSAGES")
		log.Infof("WARNING: both can be found in the helm chart for each service.")
	}
	if errors.Is(err, ErrPeekLockTimeout) {
		log.Infof("WARNING: processing msg %d duration %s returned error: %v", count, duration, err)
		log.Infof("WARNING: please enable SERVICEBUS_RENEW_LOCK which can be found in the helm chart")
	}
}

// processMessage disposes of message and emits 2 log messages detailing how long processing took.
func (r *Receiver) processMessage(ctx context.Context, count int, maxDuration time.Duration, msg *ReceivedMessage, handler Handler) {
	now := time.Now()

	// the context wont have a trace span on it yet, so stick with the reciever logger instance

	r.log.Debugf("Processing message %d", count)
	disp, ctx, err := r.handleReceivedMessageWithTracingContext(ctx, msg, handler)
	r.dispose(ctx, disp, err, msg)

	duration := time.Since(now)

	// Now we do have a tracing context we can use it for logging
	log := r.log.FromContext(ctx)
	defer log.Close()

	log.Debugf("Processing message %d took %s", count, duration)

	// This is safe because maxDuration is only defined if RenewMessageLock is false.
	if !r.Cfg.RenewMessageLock && duration >= maxDuration {
		log.Infof("WARNING: processing msg %d duration %v took more than %v seconds", count, duration, maxDuration)
		log.Infof("WARNING: please either enable SERVICEBUS_RENEW_LOCK or reduce SERVICEBUS_INCOMING_MESSAGES")
		log.Infof("WARNING: both can be found in the helm chart for each service.")
	}
	if errors.Is(err, ErrPeekLockTimeout) {
		log.Infof("WARNING: processing msg %d duration %s returned error: %v", count, duration, err)
		log.Infof("WARNING: please enable SERVICEBUS_RENEW_LOCK which can be found in the helm chart")
	}
}

// renewMessageLock renews the given messages peek lock, so it doesn't lose the lock and get re-added to the message queue.
//
// Stop the message lock renewal by cancelling the passed in context
func (r *Receiver) renewMessageLock(ctx context.Context, count int, msg *ReceivedMessage) {
	var err error

	ticker := time.NewTicker(r.Cfg.RenewMessageTime)

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

func (r *Receiver) receiveMessagesInParallel() error {

	numberOfReceivedMessages := len(r.handlers)
	r.log.Debugf(
		"NumberOfReceivedMessages %d, RenewMessageLock: %v",
		numberOfReceivedMessages,
		r.Cfg.RenewMessageLock,
	)

	// Start all the workers
	msgs := make(chan *ReceivedMessage, numberOfReceivedMessages)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for i := range numberOfReceivedMessages {
		go func(rctx context.Context, ii int, rr *Receiver) {
			rr.log.Debugf("Start worker %d", ii)
			for {
				select {
				case <-rctx.Done():
					rr.log.Debugf("Stop worker %d", ii)
					return
				case msg := <-msgs:
					var renewCtx context.Context
					var renewCancel context.CancelFunc
					var maxDuration time.Duration
					if rr.Cfg.RenewMessageLock {
						renewCtx, renewCancel = context.WithCancel(rctx)
						go rr.renewMessageLock(renewCtx, ii+1, msg)
					} else {
						// we need a timeout if RenewMessageLock is disabled
						renewCtx, renewCancel, maxDuration = rr.setTimeout(rctx, rr.log, msg)
					}
					rr.processMessage(renewCtx, ii+1, maxDuration, msg, rr.handlers[ii])
					renewCancel()
					wg.Done()
				}
			}
		}(ctx, i, r)
	}

	// Extensively tested by loading messages and checking that the waitGroup logic always reset to zero so messages
	// continue to be processed. The sync.Waitgroup will panic if the internal counter ever goes to less than zero - this is
	// what we want as then the service will restart. Extensive testing has never encountered this.
	// The load tests wree conducted with over 1000 simplhash anchor messages present and with NumberOfReceivedMessage=8.
	// The code mosly read 8 messages at a time - sometimes only 3 or 4 were read - either way the code processed the
	// messages successfully and only finished once the receiver was empty.
	for {
		var err error
		var messages []*ReceivedMessage
		messages, err = r.receiver.ReceiveMessages(ctx, numberOfReceivedMessages, nil)
		if err != nil {
			azerr := fmt.Errorf("%s: ReceiveMessage failure: %w", r, NewAzbusError(err))
			r.log.Infof("%s", azerr)
			return azerr
		}
		total := len(messages)
		r.log.Debugf("total messages %d", total)

		for i := range total {
			wg.Add(1)
			msgs <- messages[i]
		}
		wg.Wait()
		r.log.Debugf("Processed %d messages", total)
	}
}

func (r *Receiver) receiveMessagesInSerial() error {

	r.log.Debugf(
		"NumberOfReceivedMessages %d, RenewMessageLock: %v",
		r.numberOfReceivedMessages,
		r.Cfg.RenewMessageLock,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		var err error
		var messages []*ReceivedMessage
		messages, err = r.receiver.ReceiveMessages(ctx, r.numberOfReceivedMessages, nil)
		if err != nil {
			azerr := fmt.Errorf("%s: ReceiveMessage failure: %w", r, NewAzbusError(err))
			r.log.Infof("%s", azerr)
			return azerr
		}
		total := len(messages)
		r.log.Debugf("total messages %d", total)
		var renewCtx context.Context
		var renewCancel context.CancelFunc
		var maxDuration time.Duration
		// XXX: if the number of Received Messages is large (>10) then RenewMessageLock is required.
		if r.Cfg.RenewMessageLock {
			func() {
				renewCtx, renewCancel = context.WithCancel(ctx)
				defer renewCancel()
				for i, msg := range messages {
					go r.renewMessageLock(renewCtx, i+1, msg)
				}
				for i, msg := range messages {
					r.processMessage(renewCtx, i+1, maxDuration, msg, r.serialHandler)
				}
			}()
		} else {
			for i, msg := range messages {
				func() {
					// we need a timeout per message if RenewMessageLock is disabled
					renewCtx, renewCancel, maxDuration = r.setTimeout(ctx, r.log, msg)
					defer renewCancel()
					r.processMessage(renewCtx, i+1, maxDuration, msg, r.serialHandler)
				}()
			}
		}
	}
}

func (r *Receiver) receiveMessagesInBatch() error {

	r.log.Debugf(
		"NumberOfReceivedMessages %d, RenewMessageLock: %v",
		r.numberOfReceivedMessages,
		r.Cfg.RenewMessageLock,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		var err error
		var messages []*ReceivedMessage
		messages, err = r.receiver.ReceiveMessages(ctx, r.numberOfReceivedMessages, nil)
		if err != nil {
			azerr := fmt.Errorf("%s: ReceiveMessage failure: %w", r, NewAzbusError(err))
			r.log.Infof("%s", azerr)
			return azerr
		}
		total := len(messages)
		r.log.Debugf("total messages %d", total)
		var renewCtx context.Context
		var renewCancel context.CancelFunc
		var maxDuration time.Duration
		// XXX: if the number of Received Messages is large (>10) then RenewMessageLock is required.
		func() {
			if r.Cfg.RenewMessageLock {
				renewCtx, renewCancel = context.WithCancel(ctx)
				defer renewCancel()
				for i, msg := range messages {
					go r.renewMessageLock(renewCtx, i+1, msg)
				}
			} else {
				// we need a timeout per message if RenewMessageLock is disabled - use first message
				// for all messages
				renewCtx, renewCancel, maxDuration = r.setTimeout(ctx, r.log, messages[0])
				defer renewCancel()
			}
			r.processMessages(renewCtx, maxDuration, messages, r.batchHandler)
		}()
	}
}

// The following 2 methods satisfy the startup.Listener interface.
func (r *Receiver) Listen() error {
	r.log.Debugf("listen")
	err := r.open()
	if err != nil {
		azerr := fmt.Errorf("%s: ReceiveMessage failure: %w", r, NewAzbusError(err))
		r.log.Infof("%s", azerr)
		return azerr
	}
	if r.batchHandler != nil {
		return r.receiveMessagesInBatch()
	}
	if r.serialHandler != nil {
		return r.receiveMessagesInSerial()
	}
	return r.receiveMessagesInParallel()
}

func (r *Receiver) Shutdown(ctx context.Context) error {
	r.close_()
	return nil
}

func (r *Receiver) open() error {
	var err error

	if r.receiver != nil {
		return nil
	}

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

	switch {
	case r.batchHandler != nil:
		err = r.batchHandler.Open()
		if err != nil {
			return fmt.Errorf("failed to open batch handler: %w", err)
		}
	case r.serialHandler != nil:
		err = r.serialHandler.Open()
		if err != nil {
			return fmt.Errorf("failed to open serial handler: %w", err)
		}
	default:
		for j := range len(r.handlers) {
			err = r.handlers[j].Open()
			if err != nil {
				return fmt.Errorf("failed to open handler: %w", err)
			}
		}
	}
	return nil
}

func (r *Receiver) close_() {
	if r != nil {
		if r.receiver != nil {
			r.mtx.Lock()
			defer r.mtx.Unlock()

			err := r.receiver.Close(context.Background())
			if err != nil {
				azerr := fmt.Errorf("%s: Error closing receiver: %w", r, NewAzbusError(err))
				r.log.Infof("%s", azerr)
			}
			for j := range len(r.handlers) {
				r.handlers[j].Close()
			}
			r.handlers = []Handler{}
			if r.serialHandler != nil {
				r.serialHandler.Close()
				r.serialHandler = nil
			}
			if r.batchHandler != nil {
				r.batchHandler.Close()
				r.batchHandler = nil
			}
			r.receiver = nil
		}
	}
}
