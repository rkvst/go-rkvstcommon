package azbus

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

type MsgSender interface {
	Open() error
	Close(context.Context)

	Send(context.Context, *OutMessage) error
	NewMessageBatch(context.Context) (*OutMessageBatch, error)
	BatchAddMessage(batch *OutMessageBatch, m *OutMessage, options *azservicebus.AddMessageOptions) error

	SendBatch(context.Context, *OutMessageBatch) error
	String() string
}
