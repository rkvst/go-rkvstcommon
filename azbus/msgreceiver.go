package azbus

import (
	"context"
)

type MsgReceiver interface {
	Open() error
	Close(context.Context)
	String() string

	// Listener interface
	Listen() error
	Shutdown(context.Context) error

	GetAZClient() AZClient
}
