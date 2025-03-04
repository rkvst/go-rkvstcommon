package azbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/google/uuid"
)

// SenderConfig configuration for an azure servicebus namespace and queue
type SenderConfig struct {
	ConnectionString string

	// Name is the name of the queue or topic to send to.
	TopicOrQueueName string
}

// Sender to send or receive messages on  a queue or topic
type Sender struct {
	azClient AZClient

	Cfg SenderConfig

	log                   Logger
	mtx                   sync.Mutex
	sender                *azservicebus.Sender
	maxMessageSizeInBytes int64
	spanner               Spanner
}

// NewSender creates a new client
func NewSender(log Logger, cfg SenderConfig) *Sender {

	s := &Sender{
		Cfg:      cfg,
		azClient: NewAZClient(cfg.ConnectionString),
	}
	s.log = log.WithIndex("sender", s.String())
	s.spanner = NewSpanning(s.log)
	return s
}

func (s *Sender) String() string {
	return s.Cfg.TopicOrQueueName
}

func (s *Sender) Close(ctx context.Context) {

	var err error
	if s != nil && s.sender != nil {
		s.log.Debugf("Close")
		s.mtx.Lock()
		defer s.mtx.Unlock()
		err = s.sender.Close(ctx)
		if err != nil {
			azerr := fmt.Errorf("%s: Error closing sender: %w", s, NewAzbusError(err))
			s.log.Infof("%s", azerr)
		}
		s.sender = nil // not going to attempt to close again on error
	}
	if s != nil && s.spanner != nil {
		s.spanner.Close()
		s.spanner = nil
	}
}

func (s *Sender) Open() error {
	var err error

	if s.sender != nil {
		return nil
	}

	client, err := s.azClient.azClient()
	if err != nil {
		return err
	}

	azadmin := newazAdminClient(s.log, s.Cfg.ConnectionString)
	s.maxMessageSizeInBytes, err = azadmin.getQueueMaxMessageSize(s.Cfg.TopicOrQueueName)
	if err != nil {
		azerr := fmt.Errorf("%s: failed to get sender properties: %w", s, NewAzbusError(err))
		s.log.Infof("%s", azerr)
		return azerr
	}
	s.log.Debugf("Maximum message size is %d bytes", s.maxMessageSizeInBytes)

	sender, err := client.NewSender(s.Cfg.TopicOrQueueName, nil)
	if err != nil {
		azerr := fmt.Errorf("%s: failed to open sender: %w", s, NewAzbusError(err))
		s.log.Infof("%s", azerr)
		return azerr
	}

	s.log.Debugf("Open")
	s.sender = sender
	return nil
}

func (s *Sender) updateSendingMesssage(ctx context.Context, log Logger, message *OutMessage) {

	var err error
	var attrs map[string]string
	log.Debugf("updateSendingMesssage(): context %#v", ctx)
	if s.spanner != nil {
		attrs, err = s.spanner.InjectContext(ctx)
		if err != nil {
			log.Infof("updateSendingMesssage(): failed to inject: %v", err)
		}
	}
	//if attrs == nil {
	//}
	for k, v := range attrs {
		log.Debugf("updateSendingMesssage(): %s, %v", k, v)
		OutMessageSetProperty(message, k, v)
	}
	log.Debugf("updateSendingMesssageForSpan(): ApplicationProperties %v", OutMessageProperties(message))
}

// Send submits a message to the queue. Ignores cancellation.
func (s *Sender) Send(ctx context.Context, message *OutMessage) error {

	// Without this fix eventsourcepoller and similar services repeatedly context cancel and repeatedly
	// restart.
	ctx = context.WithoutCancel(ctx)
	var log Logger
	var err error

	id := uuid.New().String()
	message.MessageID = &id

	if s.spanner != nil {
		ctx = s.spanner.Open(
			ctx,
			"Sender.Send",
			[2]string{"sender", s.Cfg.TopicOrQueueName},
			[2]string{"message.id", id},
		)
		defer s.spanner.Close()
		log = s.spanner.Log()
	} else {
		log = s.log.FromContext(ctx)
		defer log.Close()
	}
	// boots & braces
	if s.sender == nil {
		err = s.Open()
		if err != nil {
			return err
		}
	}

	size := int64(len(message.Body))
	log.Debugf("%s: Msg id %s Sized %d limit %d", s, id, size, s.maxMessageSizeInBytes)
	if size > s.maxMessageSizeInBytes {
		log.Debugf("Msg Sized %d > limit %d :%v", size, s.maxMessageSizeInBytes, ErrMessageOversized)
		return fmt.Errorf("%s: Msg Sized %d > limit %d :%w", s, size, s.maxMessageSizeInBytes, ErrMessageOversized)
	}
	now := time.Now()

	s.updateSendingMesssage(ctx, log, message)

	err = s.sender.SendMessage(ctx, message, nil)
	if err != nil {
		azerr := fmt.Errorf("Send message id %s failed in %s: %w", id, time.Since(now), NewAzbusError(err))
		log.Infof("%s", azerr)
		return azerr
	}
	log.Debugf("Sending message id %s took %s", id, time.Since(now))
	return nil
}

func (s *Sender) NewMessageBatch(ctx context.Context) (*OutMessageBatch, error) {
	return s.sender.NewMessageBatch(ctx, nil)
}

// BatchAddMessage calls Addmessage on batch
// Note: this method is a direct pass through and exists only to provide a
// mockable interface for adding messages to a batch.
func (s *Sender) BatchAddMessage(batch *OutMessageBatch, m *OutMessage, options *azservicebus.AddMessageOptions) error {
	return batch.AddMessage(m, options)
}

// SendBatch submits a message batch to the broker. Ignores cancellation.
func (s *Sender) SendBatch(ctx context.Context, batch *OutMessageBatch) error {

	// Without this fix eventsourcepoller and similar services repeatedly context cancel and repeatedly
	// restart.
	ctx = context.WithoutCancel(ctx)

	var log Logger
	var err error

	now := time.Now()

	if s.spanner != nil {
		ctx = s.spanner.Open(
			ctx,
			"Sender.SendBatch",
			[2]string{"sender", s.Cfg.TopicOrQueueName},
		)
		defer s.spanner.Close()
		log = s.spanner.Log()
	} else {
		log = s.log.FromContext(ctx)
		defer log.Close()
	}
	// boots & braces
	if s.sender == nil {
		err = s.Open()
		if err != nil {
			return err
		}
	}
	// Note: sizing must be dealt with as the batch is created and accumulated.

	// Note: the first message properties (including application properties) are established by the first message in the batch

	err = s.sender.SendMessageBatch(ctx, batch, nil)
	if err != nil {
		azerr := fmt.Errorf("SendMessageBatch failed in %s: %w", time.Since(now), NewAzbusError(err))
		log.Infof("%s", azerr)
		return azerr
	}
	return nil
}
