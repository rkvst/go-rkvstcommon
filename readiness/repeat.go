package readiness

// For utilities that assist checking if other things are ready or repeating
// things until they are.

import (
	"errors"
	"time"

	"github.com/datatrails/go-datatrails-common/logger"
)

// Repeat repeatedly calls func until it returns without a recoverable error
// or attempts are exhausted. attempts = -1 to try forever. interval is the delay between
// attempts.
func Repeat(attempts int, interval time.Duration, f func() error) error {
	var err error

	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return nil
		}

		// exit early if error is unrecoverable
		var e *UnrecoverableError
		if errors.As(err, &e) {
			return err
		}

		if attempts > -1 && i >= (attempts-1) {
			break
		}
		logger.Sugar.Debugw(
			"retrying ...",
			"count", i, "interval", interval, "err", err)

		time.Sleep(interval)
	}

	return err
}
