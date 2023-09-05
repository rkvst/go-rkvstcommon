package errhandling

import (
	"errors"
	"fmt"
	"testing"

	"github.com/rkvst/go-rkvstcommon/logger"
	"github.com/stretchr/testify/assert"
)

// TestErrB2c
//
// Tests we can successfully make a B2C error as well
//
//	as make assertions into whether an error is a B2C error.
func TestErrB2c(t *testing.T) {
	logger.New("DEBUG")

	table := []struct {
		name     string
		err      error
		expected *ErrB2C
		eErr     error
	}{
		{
			"positive",
			&ErrB2C{
				APIVersion:  "1.0.0",
				Status:      404,
				UserMessage: "foo bar",
			},
			&ErrB2C{
				APIVersion:  "1.0.0",
				Status:      404,
				UserMessage: "foo bar",
			},
			nil,
		},
		{
			"grpc wrapped error",
			fmt.Errorf(grpcErrPrefix+b2cFmtString, "1.0.0", 404, "foo bar"),
			&ErrB2C{
				APIVersion:  "1.0.0",
				Status:      404,
				UserMessage: "foo bar",
			},
			nil,
		},
		{
			"wrong error",
			errors.New("im no b2c error"),
			nil,
			errors.New("im no b2c error"),
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			actual, err := GetErrB2c(test.err)

			assert.Equal(t, test.eErr, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}

// TestB2CErrError
//
// Tests we get the correct Error() from a B2C error.
func TestB2CErrError(t *testing.T) {

	expected := fmt.Errorf(b2cFmtString, "1.0.0", 404, "foo bar")

	b2cErr := &ErrB2C{
		APIVersion:  "1.0.0",
		Status:      404,
		UserMessage: "foo bar",
	}

	assert.Equal(t, expected.Error(), b2cErr.Error())
}
