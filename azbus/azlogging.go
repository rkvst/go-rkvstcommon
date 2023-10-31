package azbus

import (
	azlog "github.com/Azure/azure-sdk-for-go/sdk/azcore/log"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// First attempt at incorporating azure logging. The EventSender option does not
// appear to work.
// TODO: Generalise this for any azure downstream package.
func EnableAzureLogging(log Logger) {
	log.Debugf("Enabling Azure Logging")
	azlog.SetListener(func(event azlog.Event, s string) {
		log.Debugf("[%s] %s", event, s)
	})

	azlog.SetEvents(
		// EventConn is used whenever we create a connection or any links (ie: receivers, senders).
		// azservicebus.EventConn,
		// EventAuth is used when we're doing authentication/claims negotiation.
		// azservicebus.EventAuth,
		// EventReceiver represents operations that happen on Receivers.
		azservicebus.EventReceiver,
		// EventSender represents operations that happen on Senders.
		azservicebus.EventSender,
		// EventAdmin is used for operations in the azservicebus/admin.Client
		// azservicebus.EventAdmin,
	)
}
