package azbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// BatchHandler is completely responsible for the processing of a batch of messages.
// Implementations take complete responsibility for the peek lock renewal and disposal of messages.
type BatchHandler interface {
	Handle(context.Context, Disposer, []*ReceivedMessage) error
	Open() error
	Close()
}

// BatchRecieverConfig provides the configuration for receivers that work with azure batched send
// * There is not autmatic message lock renewal provision for the batched receiver
// * There is no support for deadletter queues on the batched receiver
type BatchReceiverConfig struct {
	ConnectionString string

	// Name is the name of the queue or topic
	TopicOrQueueName string

	// Subscriptioon is the name of the topic subscription.
	// If blank then messages are received from a Queue.
	SubscriptionName string

	// If a deadletter receiver then this is true
	Deadletter bool

	// Receive messages in a batch and completely delegate processing to a single dedicated handler
	BatchSize int

	// A batch operation must abandon any message that takes longer than this to process.
	// Defaults to DefaultRenewalTime.
	BatchDeadline time.Duration
}

// BatchReceiver to receive messages on  a queue
type BatchReceiver struct {
	azClient AZClient

	Cfg BatchReceiverConfig

	log      Logger
	mtx      sync.Mutex
	Receiver *azservicebus.Receiver
	Options  *azservicebus.ReceiverOptions
	Handler  BatchHandler
	Cancel   context.CancelFunc
	spanner  NewSpanFunc
}

type BatchReceiverOption func(*BatchReceiver)

// WithBatchDeadline sets the context deadline used for the batch operation.
// If this is not set, the default is DefaultRenewalTime.
func WithBatchDeadline(d time.Duration) BatchReceiverOption {
	return func(r *BatchReceiver) {
		r.Cfg.BatchDeadline = d
	}
}

// NewBatchReceiver creates a new BatchReceiver.
func NewBatchReceiver(log Logger, handler BatchHandler, cfg BatchReceiverConfig, opts ...BatchReceiverOption) *BatchReceiver {
	r := BatchReceiver{}
	var options *azservicebus.ReceiverOptions

	r.Cfg = cfg
	r.spanner = NewSpan
	r.azClient = NewAZClient(cfg.ConnectionString)
	r.Options = options
	r.Handler = handler
	r.log = log.WithIndex("receiver", r.String())
	for _, opt := range opts {
		opt(&r)
	}

	if r.Cfg.BatchDeadline == 0 {
		r.Cfg.BatchDeadline = DefaultRenewalTime
	}

	return &r
}

// String - returns string representation of receiver.
func (r *BatchReceiver) String() string {
	// No log function calls in this method please.
	if r.Cfg.SubscriptionName != "" {
		return fmt.Sprintf("%s.%s", r.Cfg.TopicOrQueueName, r.Cfg.SubscriptionName)
	}
	return fmt.Sprintf("%s", r.Cfg.TopicOrQueueName)
}

func (r *BatchReceiver) receiveMessages(ctx context.Context) error {
	r.log.Debugf("BatchSize %d, BatchDeadline: %v", r.Cfg.BatchSize, r.Cfg.BatchDeadline)

	for {
		err := r.receiveOneMessageBatch(ctx)
		if err != nil {
			return err
		}
	}
}

func (r *BatchReceiver) receiveOneMessageBatch(ctx context.Context) error {

	if r.Cfg.BatchSize == 0 {
		return fmt.Errorf("BatchSize must be greater than zero")
	}

	var err error
	var messages []*ReceivedMessage
	messages, err = r.Receiver.ReceiveMessages(ctx, r.Cfg.BatchSize, nil)
	if err != nil {
		azerr := fmt.Errorf("%s: ReceiveMessage failure: %w", r, NewAzbusError(err))
		r.log.Infof("%s", azerr)
		return azerr
	}
	total := len(messages)
	r.log.Debugf("total messages %d", total)

	// set a deadline for the batch operation, this should be shorter than the peak lock timeout
	ctx, cancel := context.WithTimeout(ctx, r.Cfg.BatchDeadline)
	defer cancel()

	// creating the attributes from the first message is a bit arbitrary, but it's the best we can do
	var span Spanner
	span, ctx, _ = r.spanner(
		ctx,
		r.log,
		"BatchReceive",
		map[string]any{
			"receiver": r.Cfg.TopicOrQueueName,
		},
		messages[0].ApplicationProperties,
	)
	defer span.Close()

	err = r.Handler.Handle(ctx, r, messages)
	if err != nil {
		r.log.Infof("terminating due to batch handler err: %v", err)
		return err
	}

	r.log.Debugf("Processed %d messages", total)

	return nil
}

// The following 2 methods satisfy the startup.Listener interface.
func (r *BatchReceiver) Listen() error {
	ctx, cancel := context.WithCancel(context.Background())
	r.Cancel = cancel
	r.log.Debugf("listen")
	err := r.open()
	if err != nil {
		azerr := fmt.Errorf("%s: ReceiveMessage failure: %w", r, NewAzbusError(err))
		r.log.Infof("%s", azerr)
		return azerr
	}
	return r.receiveMessages(ctx)
}

func (r *BatchReceiver) Shutdown(ctx context.Context) error {
	r.Cancel()
	r.close_()
	return nil
}

func (r *BatchReceiver) open() error {
	var err error

	if r.Receiver != nil {
		return nil
	}

	client, err := r.azClient.azClient()
	if err != nil {
		return err
	}

	var receiver *azservicebus.Receiver
	if r.Cfg.SubscriptionName != "" {
		receiver, err = client.NewReceiverForSubscription(r.Cfg.TopicOrQueueName, r.Cfg.SubscriptionName, r.Options)
	} else {
		receiver, err = client.NewReceiverForQueue(r.Cfg.TopicOrQueueName, r.Options)
	}
	if err != nil {
		azerr := fmt.Errorf("%s: failed to open receiver: %w", r, NewAzbusError(err))
		r.log.Infof("%s", azerr)
		return azerr
	}

	r.Receiver = receiver
	if r.Handler != nil {
		err = r.Handler.Open()
		if err != nil {
			return fmt.Errorf("failed to open batch handler: %w", err)
		}
	}
	return nil
}

func (r *BatchReceiver) close_() {
	if r == nil {
		return
	}
	r.log.Debugf("Close")
	if r.Receiver != nil {
		r.mtx.Lock()
		defer r.mtx.Unlock()
		if r.Handler != nil {
			r.log.Debugf("Close batch handler")
			r.Handler.Close()
			r.Handler = nil
		}

		r.log.Debugf("Close receiver")
		err := r.Receiver.Close(context.Background())
		if err != nil {
			azerr := fmt.Errorf("%s: Error closing receiver: %w", r, NewAzbusError(err))
			r.log.Infof("%s", azerr)
		}
		r.Handler = nil
		r.Receiver = nil
		r.Cancel = nil
	}
}
