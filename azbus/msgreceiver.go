package azbus

import (
	"context"
)

type MsgReceiver interface {
	// Listener interface
	Listen() error
	Shutdown(context.Context) error

	String() string
}
