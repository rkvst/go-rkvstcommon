package azbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	otlog "github.com/opentracing/opentracing-go/log"

	"github.com/datatrails/go-datatrails-common/tracing"
)

// so we dont have to import the azure repo everywhere
type OutMessage = azservicebus.Message

func NewOutMessage(data []byte) OutMessage {
	return azservicebus.Message{
		Body: data,
	}
}

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
}

// NewSender creates a new client
func NewSender(log Logger, cfg SenderConfig) *Sender {

	s := &Sender{
		Cfg:      cfg,
		azClient: NewAZClient(cfg.ConnectionString),
	}
	s.log = log.WithIndex("sender", s.String())
	return s
}

func (s *Sender) String() string {
	return s.Cfg.TopicOrQueueName
}

func (s *Sender) GetAZClient() AZClient {
	return s.azClient
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

	azadmin := NewAZAdminClient(s.log, s.Cfg.ConnectionString)
	s.maxMessageSizeInBytes, err = azadmin.GetQueueMaxMessageSize(s.Cfg.TopicOrQueueName)
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

type OutMessageOption func(*OutMessage)

func WithProperty(key string, value any) OutMessageOption {
	return func(o *OutMessage) {
		o.ApplicationProperties[key] = value
	}
}

func (s *Sender) Send(ctx context.Context, msg []byte, opts ...OutMessageOption) error {
	return s.SendMsg(ctx, NewOutMessage(msg), opts...)
}

// Send submits a message to the queue. Ignores cancellation.
func (s *Sender) SendMsg(ctx context.Context, message OutMessage, opts ...OutMessageOption) error {

	// Without this fix eventsourcepoller and similar services repeatedly context cancel and repeatedly
	// restart.
	ctx = context.WithoutCancel(ctx)

	var err error

	span, ctx := tracing.StartSpanFromContext(ctx, "Sender.Send")
	defer span.Finish()
	span.LogFields(
		otlog.String("sender", s.Cfg.TopicOrQueueName),
	)

	// Get the logging context after we create the span as that may have created a new
	// trace and stashed the traceid in the metadata.
	log := s.log.FromContext(ctx)
	defer log.Close()

	err = s.Open()
	if err != nil {
		return err
	}

	size := int64(len(message.Body))
	log.Debugf("%s: Msg Sized %d limit %d", s, size, s.maxMessageSizeInBytes)
	if size > s.maxMessageSizeInBytes {
		log.Debugf("Msg Sized %d > limit %d :%v", size, s.maxMessageSizeInBytes, ErrMessageOversized)
		return fmt.Errorf("%s: Msg Sized %d > limit %d :%w", s, size, s.maxMessageSizeInBytes, ErrMessageOversized)
	}
	now := time.Now()
	if message.ApplicationProperties == nil {
		message.ApplicationProperties = make(map[string]any)
	}
	for _, opt := range opts {
		opt(&message)
	}
	s.UpdateSendingMesssageForSpan(ctx, &message, span)

	err = s.sender.SendMessage(ctx, &message, nil)
	if err != nil {
		azerr := fmt.Errorf("Send failed in %s: %w", time.Since(now), NewAzbusError(err))
		log.Infof("%s", azerr)
		return azerr
	}
	log.Debugf("Sending message took %s", time.Since(now))
	return nil
}
