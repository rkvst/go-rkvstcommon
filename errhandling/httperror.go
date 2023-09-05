package errhandling

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HTTPError error type with info about http.StatusCode
type HTTPError interface {
	Error() string
	StatusCode() int
}

type ErrorWithStatus struct {
	err        error
	statusCode int
}

func NewErrorStatus(err error, statusCode int) *ErrorWithStatus {
	return &ErrorWithStatus{
		err:        err,
		statusCode: statusCode,
	}
}

func (e *ErrorWithStatus) StatusCode() int {
	return e.statusCode
}

func (e *ErrorWithStatus) Error() string {
	return e.err.Error()
}

func (e *ErrorWithStatus) Unwrap() error {
	return e.err
}

func HTTPErrorFromGrpcError(err error) int {
	s, ok := status.FromError(err)
	if !ok {
		s = status.New(codes.Unknown, err.Error())
	}
	return runtime.HTTPStatusFromCode(s.Code())
}
