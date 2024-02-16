package azbus

import (
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

type ReceivedMessage = azservicebus.ReceivedMessage

func ReceivedProperties(r *ReceivedMessage) map[string]any {
	if r.ApplicationProperties != nil {
		return r.ApplicationProperties
	}
	return make(map[string]any)
}

// SetProperty adds key-value pair to Message and can be chained.
func ReceivedSetProperty(r *ReceivedMessage, k string, v any) {
	r.ApplicationProperties[k] = v
}
