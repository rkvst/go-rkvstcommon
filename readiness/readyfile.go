package readiness

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/datatrails/go-datatrails-common/logger"
)

// ReadSmallFile polls attempting to read a small file and returns its entire contents
func ReadSmallFile(filename string, tries int) (string, error) {

	contents := ""
	err := Repeat(tries, 2*time.Second, func() error {
		logger.Plain.Info("Trying to read file")

		var b []byte
		var err error
		if b, err = os.ReadFile(filename); err != nil {
			logger.Sugar.Infow("file not available", "filename", filename)
			return err
		}
		contents = string(b)
		return nil
	})

	if err != nil {
		return "", errors.New("could not read file:" + filename)
	}
	return strings.TrimSpace(contents), nil
}
