package azbus

import (
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// OutMessage abstracts the output message interface.
type OutMessage = azservicebus.Message

// We dont use With style options as this is executed in the hotpath.
func NewOutMessage(data []byte) *OutMessage {
	var o OutMessage
	return newOutMessage(&o, data)
}

// function outlining
func newOutMessage(o *OutMessage, data []byte) *OutMessage {
	o.Body = data
	o.ApplicationProperties = make(map[string]any)
	return o
}

// SetProperty adds key-value pair to OutMessage and can be chained.
func OutMessageSetProperty(o *OutMessage, k string, v any) {
	o.ApplicationProperties[k] = v
}

func OutMessageProperties(o *OutMessage) map[string]any {
	if o.ApplicationProperties != nil {
		return o.ApplicationProperties
	}
	return make(map[string]any)
}
