package errhandling

import (
	"errors"
	"fmt"
	"strings"

	"github.com/datatrails/go-datatrails-common/logger"
)

const (

	// grpcErrPrefix is the prefix grpc gives to errors whose format it doesn't recognise
	grpcErrPrefix = "rpc error: code = Unknown desc = "

	// b2cAPIVersion is the api version of b2c that we are using
	b2cAPIVersion = "1.0.0"
	b2cFmtString  = "apiVersion: %s status: %d userMessage: %s"
)

type ErrB2C struct {
	// APIVersion is the API version of b2c that we are using
	APIVersion string `json:"version"`

	// Status, http status code. The REST API should return an HTTP 4xx error message
	Status int `json:"status"`

	// UserMessage is the message to the user
	UserMessage string `json:"userMessage"`
}

// NewErrB2c creates a B2C error given a http status and a user message.
func NewErrB2c(status int, format string, a ...any) *ErrB2C {
	return &ErrB2C{
		APIVersion:  b2cAPIVersion,
		Status:      status,
		UserMessage: fmt.Sprintf(format, a...),
	}
}

// WithErrorString converts a B2C error string into
//
//	a b2c error.
func (e *ErrB2C) WithErrorString(errString string) error {

	// sscan is a reverse sprintf (however we cannot have spaces in %s strings)
	if _, scanErr := fmt.Sscanf(errString, b2cFmtString,
		&e.APIVersion, &e.Status, &e.UserMessage); scanErr != nil {

		logger.Sugar.Infof("scan error: %v", scanErr)
		return scanErr
	}

	// scanF ends on spaces, so ensure we have a correct user message
	e.UserMessage = strings.Split(errString, "userMessage: ")[1]

	return nil
}

// GetErrB2c attempts to get a ErrB2c from an error
func GetErrB2c(err error) (*ErrB2C, error) {

	errB2C := &ErrB2C{}

	// already a b2c error.
	ok := errors.As(err, &errB2C)
	if ok {
		return errB2C, nil
	}

	// chance it is a status.Error (can't access status.Error as its part of an internal package)
	if !strings.HasPrefix(err.Error(), grpcErrPrefix) {
		logger.Sugar.Infof("error is not b2c error: %v", err)
		return nil, err
	}

	// can't use errors.Unwrap because status.Error doesn't implement Unwrap method
	b2cErrString := strings.TrimPrefix(err.Error(), grpcErrPrefix)

	if convErr := errB2C.WithErrorString(b2cErrString); convErr != nil {
		logger.Sugar.Infof("unable to add error string: %v", convErr)
		return nil, convErr
	}

	// gots the b2c error all good
	return errB2C, nil
}

// Error implements the err interface
func (e *ErrB2C) Error() string {
	return fmt.Sprintf(b2cFmtString, e.APIVersion, e.Status, e.UserMessage)
}
