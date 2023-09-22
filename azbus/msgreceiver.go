package azbus

import (
	"context"
)

type MsgReceiver interface {
	Open() error
	Close(context.Context)
	ReceiveMessages(Handler) error
	String() string

	// Listener interface
	Listen() error
	Shutdown(context.Context) error

	GetAZClient() AZClient

	Abandon(context.Context, error, *ReceivedMessage) error
	Reschedule(context.Context, error, *ReceivedMessage) error
	DeadLetter(context.Context, error, *ReceivedMessage) error
	Complete(context.Context, *ReceivedMessage) error
}
