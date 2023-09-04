package azbus

import (
	"context"
	"errors"
	"time"
)

// Set a timeout for processing the message, this should be no later than
// the message lock time. It is quite surprising that the azure service bus package
// does not add a deadline to the context input to the message handler.
//
// NB: this has no effect as cancellaton is removed to get the azure sdk for go retry
//
//	logic which increases reliability.
//
// Inspection of logs shows that the deadline is always 60s in the future which we will
// never exceed.
//
// The use of the context returned here is problematic. Inspection of code that uses it
// shows that submethods called do not generally obey cancellation - they do not even have
// a context.Context as first argument.
//
// Code that follows from calling this method should be wrapped in a select statement
// that terminates when the timeout expires - i.e. waits on ctx.Done(). Even this is
// not bulletproof as it is unclear how to terminate any of these submethods.
//
// Probably the best solution is to remove this entirely and rely on RenewMessageLock.
// If it does timeout then it is too late anyway as the peeklock will already be released.
//
// for the time being we impose a timeout as it is safe.
var (
	ErrPeekLockTimeout = errors.New("peeklock deadline reached")
)

func setTimeout(ctx context.Context, log Logger, msg *ReceivedMessage) (context.Context, context.CancelFunc, time.Duration) {

	var cancel context.CancelFunc

	log.Debugf("msg locked until %s", msg.LockedUntil)
	if msg.LockedUntil != nil {
		msgLockedUntil := *msg.LockedUntil
		ctx, cancel = context.WithDeadlineCause(ctx, msgLockedUntil, ErrPeekLockTimeout)
		maxDuration := msgLockedUntil.Sub(time.Now())
		log.Debugf("msg must be processed in %s", maxDuration)
		return ctx, cancel, maxDuration
	}

	ctx, cancel = context.WithTimeoutCause(ctx, RenewalTime, ErrPeekLockTimeout)
	log.Infof("could not get lock deadline from message, using fixed timeout %v", ctx)
	return ctx, cancel, RenewalTime
}
