package azbus

import (
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// Azure package expects the user to elucidate errors like so:
//
//	    var servicebusError *azservicebus.Error
//	    if errors.As(err, &servicebusError) && servicebusError.code == azservicebus.CodeUnauthorizedAccess {
//		         ...
//
// which is rather clumsy.
//
// This code maps the internal code to an actual error so one can:
//
//	if errors.Is(err, azbus.ErrConnectionLost) {
//	    ...
//
// which is easier and more idiomatic
var (
	ErrConnectionLost     = errors.New("connection lost")
	ErrLockLost           = errors.New("lock lost")
	ErrUnauthorizedAccess = errors.New("unauthorized")
	ErrTimeout            = errors.New("timeout")
)

func NewAzbusError(err error) error {
	var servicebusError *azservicebus.Error
	if errors.As(err, &servicebusError) {
		switch servicebusError.Code {
		case azservicebus.CodeUnauthorizedAccess:
			return ErrUnauthorizedAccess
		case azservicebus.CodeConnectionLost:
			return ErrConnectionLost
		case azservicebus.CodeLockLost:
			return ErrLockLost
		case azservicebus.CodeTimeout:
			return ErrTimeout
		}
	}
	return err
}
