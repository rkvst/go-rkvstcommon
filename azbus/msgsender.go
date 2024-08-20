package azbus

import (
	"context"
)

type MsgSender interface {
	Open() error
	Close(context.Context)

	Send(context.Context, *OutMessage) error
	String() string
}
