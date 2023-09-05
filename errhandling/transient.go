package errhandling

import (
	"errors"
	"fmt"
)

type transientError struct {
	desc string
	err  error
}

func (e *transientError) Error() string {
	return e.desc + ": " + e.err.Error()
}

func (e *transientError) Unwrap() error {
	return e.err
}

// NewTransientError creates an Error object that indicates a retry is
// appropriate. It is up to the consumer to decide whether to retry based on the
// context of that consumer. Note that crashing out is effectively a retry
// unless the message is explicitly Completed or DeadLettered first.
func NewTransientError(err error) error {

	return &transientError{
		desc: "transient error",
		err:  err,
	}
}
func NewTransientErrorf(format string, a ...any) error {
	err := fmt.Errorf(format, a...)
	return NewTransientError(err)
}

// IsTransient returns true if the error was wrapped to indicate its transient.
// If this function returns true it is safe, but not required, to retry the
// operation.
func IsTransient(err error) bool {
	var target *transientError

	return errors.As(err, &target)
}
