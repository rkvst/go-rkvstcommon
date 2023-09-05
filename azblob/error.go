// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"errors"
	"net/http"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/rkvst/go-rkvstcommon/logger"
)

// HTTPError error type with info about http.StatusCode
type HTTPError interface {
	Error() string
	StatusCode() int
	Unwrap() error
}

type Error struct {
	err        error
	statusCode int
}

func NewStatusError(text string, statusCode int) *Error {
	return &Error{
		err:        errors.New(text),
		statusCode: statusCode,
	}
}

func ErrorFromError(err error) *Error {
	return &Error{err: err}
}

func (e *Error) Error() string {
	return e.err.Error()
}
func (e *Error) Unwrap() error {
	return e.err
}

// StatusCode returns status code for failing request or 500 if code is not available on the error
func (e *Error) StatusCode() int {

	var terr *azStorageBlob.StorageError
	if errors.As(e.err, &terr) {
		resp := terr.Response()
		if resp.Body != nil {
			defer resp.Body.Close()
		}
		logger.Sugar.Debugf("Azblob StatusCode %d", resp.StatusCode)
		return resp.StatusCode
	}
	if e.statusCode != 0 {
		logger.Sugar.Debugf("Return statusCode %d", e.statusCode)
		return e.statusCode
	}
	logger.Sugar.Debugf("Return InternalServerError")
	return http.StatusInternalServerError
}
