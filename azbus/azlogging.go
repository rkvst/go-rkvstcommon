package azbus

import (
	azlog "github.com/Azure/azure-sdk-for-go/sdk/azcore/log"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// EnableAzureLogging emits log messages using local logger.
// This must be called before any senders or receivers are opened.
// TODO: Generalise this for any azure downstream package.
func EnableAzureLogging(log Logger) {
	if !log.Check(DebugLevel) {
		return
	}
	log.Debugf("Enabling Azure Logging")
	azlog.SetListener(func(event azlog.Event, s string) {
		log.Debugf("Azure sdk:[%s] %s", event, s)
	})

	azlog.SetEvents(
		// EventConn is used whenever we create a connection or any links (ie: receivers, senders).
		// emits one message whenever a connectio  is established - when opening receiver or sender or
		// reestablishing a connection if retries are attempted.
		// azservicebus.EventConn,
		// EventAuth is used when we're doing authentication/claims negotiation.
		// This emits 8 messages on initial start and  8 messages every 5 minutes
		// azservicebus.EventAuth,
		// EventReceiver represents operations that happen on Receivers.
		// emits 2 log records for every message received.
		azservicebus.EventReceiver,
		// EventSender represents operations that happen on Senders.
		// emits nothing except when errors occur. An error will cause retries. Both the error
		// and the retry loop cause logs - usually this succeeds within 3 retries and 20s.
		azservicebus.EventSender,
		// EventAdmin is used for operations in the azservicebus/admin.Client
		// emits logs when creating a subscription rule
		// azservicebus.EventAdmin,
	)
}
