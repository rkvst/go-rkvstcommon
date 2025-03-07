package azbus

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"google.golang.org/grpc/metadata"
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

// SetProperty adds key-value pair to OutMessage.
func OutMessageSetProperty(o *OutMessage, k string, v any) {
	o.ApplicationProperties[k] = v
}

// SetPropertyFromMetadata adds key-value pair to OutMessage if it does not
// exist.
func OutMessageSetPropertyFromMetadata(ctx context.Context, o *OutMessage, k string) {
	value := OutMessageProperty(o, k)
	if value == "" {
		value := ValueFromContextMetadata(ctx, k)
		if value == "" {
			o.ApplicationProperties[k] = value
		}
	}
}

// Property gets string value from key to OutMessageProperties.
func OutMessageProperty(o *OutMessage, k string) string {
	if o.ApplicationProperties != nil {
		v, found := o.ApplicationProperties[k]
		if found {
			value, ok := v.(string)
			if ok {
				return value
			}
		}
	}
	return ""
}

func OutMessageProperties(o *OutMessage) map[string]any {
	if o.ApplicationProperties != nil {
		return o.ApplicationProperties
	}
	return make(map[string]any)
}

// ValueFromContextMetadata updates property from context metadata.
func ValueFromContextMetadata(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for i, v := range md {
		if strings.EqualFold(i, key) {
			return v[0]
		}
	}
	return ""
}
