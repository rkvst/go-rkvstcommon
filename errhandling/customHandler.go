package errhandling

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/datatrails/go-datatrails-common/logger"
	"google.golang.org/grpc/codes"
)

const (
	QuotaReached  = "QuotaReached"
	QuotaUnknown  = "QuotaUnknown"
	KhipuDisabled = "KhipuDisabled"
	KhipuUnknown  = "KhipuUnknown"
)

// SetQuotaReached returns a labelled QuotaReached error
func SetQuotaReached(ctx context.Context, tenantID string, name string) error {
	return StatusWithRequestInfoFromContext(
		ctx,
		StatusWithQuotaFailure(tenantID, QuotaReached, name),
	).Err()
}

// SetQuotaUnknown returns a labelled QuotaUnknown error
func SetQuotaUnknown(ctx context.Context, tenantID string, name string, err error) error {
	return StatusWithRequestInfoFromContext(
		ctx,
		StatusWithQuotaFailure(tenantID, QuotaUnknown, fmt.Sprintf("%s: %v", name, err)),
	).Err()
}

// SetKhipuUnknown returns a labelled KhipuUnknown error
func SetKhipuUnknown(ctx context.Context, tenantID string) error {
	return StatusWithRequestInfoFromContext(
		ctx,
		StatusWithQuotaFailure(tenantID, KhipuUnknown, ""),
	).Err()
}

// SetKhipuDisabled returns a labelled KhipuUnknown error
func SetKhipuDisabled(ctx context.Context, tenantID string) error {
	return StatusWithRequestInfoFromContext(
		ctx,
		StatusWithQuotaFailure(tenantID, KhipuDisabled, ""),
	).Err()
}

// NewB2CErrorHandler returns a handler for Azure format b2c errors.
//
// return a json response in the format defined here:
// https://docs.microsoft.com/en-us/azure/active-directory-b2c/restful-technical-profile#returning-validation-error-message
func NewB2CErrorHandler(fallback runtime.ErrorHandlerFunc) runtime.ErrorHandlerFunc {
	return func(
		ctx context.Context,
		mux *runtime.ServeMux,
		marshaler runtime.Marshaler,
		w http.ResponseWriter,
		r *http.Request,
		err error,
	) {

		// if its a b2c error then handle it separately
		if b2cErr, localErr := GetErrB2c(err); localErr == nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(b2cErr.Status)

			buf, localErr := json.Marshal(b2cErr)
			if localErr != nil {
				logger.Sugar.Infof("Failed to marshal b2c error response: %v", localErr)
				// if we can't make json response, just return the error string.
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				if _, writeErr := io.WriteString(w, fmt.Sprintf("message: %v", b2cErr)); writeErr != nil {
					// nothing we can do about a write error
					logger.Sugar.Infof("Failed to write b2c error response: %v", writeErr)
					fallback(ctx, mux, marshaler, w, r, err)
				}
			}

			_, writeErr := w.Write(buf)
			if writeErr != nil {
				// nothing we can do about a write error
				logger.Sugar.Infof("Failed to write b2c error response: %s: %v", string(buf), writeErr)
				fallback(ctx, mux, marshaler, w, r, err)
			}
			return
		}

		fallback(ctx, mux, marshaler, w, r, err)
	}
}

// when checking for 402 or 500 errors we must check both for wrapped errors (if the error has
// occurred in this service) or for errors that have been returned from a call to another service.
//
// In the first case the error is checked using the standard errors.Is() function.
// In the second case we parse the status error string returned from the service.

// if it is a quota error or khipu is disabled then replace http status with 402
// (pay us money...)
func checkFor402(err error) bool {
	if IsStatusLimit(err, QuotaReached) {
		return true
	}
	if IsStatusLimit(err, KhipuDisabled) {
		return true
	}
	return false
}

// if it is a unknown error (failure to retrieve the curreat caps limit or to
// determine if khipu is Enabled) then replace http status with 500
func checkFor500(err error) bool {
	if IsStatusLimit(err, QuotaUnknown) {
		return true
	}
	if IsStatusLimit(err, KhipuUnknown) {
		return true
	}
	return false
}

func NewHTTPErrorHandler(fallback runtime.ErrorHandlerFunc) runtime.ErrorHandlerFunc {
	return func(
		ctx context.Context,
		mux *runtime.ServeMux,
		marshaler runtime.Marshaler,
		w http.ResponseWriter,
		r *http.Request,
		err error,
	) {
		logger.Sugar.Debugf("Error received: %v", err)

		switch {
		case checkFor402(err):
			logger.Sugar.Debugf("Change to 402")
			httpStatus := http.StatusPaymentRequired
			err = &runtime.HTTPStatusError{
				HTTPStatus: httpStatus,
				Err: StatusError(
					ctx,
					codes.Unimplemented,
					"%v: %v", err, http.StatusText(httpStatus),
				),
			}
		case checkFor500(err):
			logger.Sugar.Debugf("Change to 500")
			httpStatus := http.StatusInternalServerError
			err = &runtime.HTTPStatusError{
				HTTPStatus: httpStatus,
				Err: StatusError(
					ctx,
					codes.Unimplemented,
					"%v: %v", err, http.StatusText(httpStatus),
				),
			}
		}

		fallback(ctx, mux, marshaler, w, r, err)
	}
}
