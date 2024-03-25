package errhandling

import (
	"context"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-common/tracing"
)

// so we dont have to import status package everywhere
type Status = status.Status

func NewStatus(c codes.Code, format string, args ...any) *Status {
	return status.Newf(c, format, args...)
}

func StatusError(ctx context.Context, c codes.Code, fmt string, opts ...any) error {
	s := NewStatus(c, fmt, opts...)
	return StatusWithRequestInfoFromContext(ctx, s).Err()
}

func StatusWithRequestInfoFromContext(ctx context.Context, s *Status) *Status {
	traceID := tracing.TraceIDFromContext(ctx)
	if traceID == "" {
		logger.Sugar.Infof("no traceID in context")
		return s
	}
	logger.Sugar.Debugf("traceID %s", traceID)
	st, err := s.WithDetails(&errdetails.RequestInfo{
		RequestId: traceID,
	})
	if err != nil {
		logger.Sugar.Infof("cannot add traceID %s: %v", traceID, err)
		return s
	}
	return st
}

func StatusWithQuotaFailure(subject string, label string, name string) *Status {
	s := NewStatus(codes.Unimplemented, "%s %s %s", subject, label, name)
	violation := errdetails.QuotaFailure_Violation{
		Subject:     subject,
		Description: " " + label + " " + name,
	}
	st, err := s.WithDetails(&errdetails.QuotaFailure{
		Violations: []*errdetails.QuotaFailure_Violation{
			&violation,
		},
	})
	if err != nil {
		logger.Sugar.Infof("cannot add quota failure %s: %v", subject, err)
		return s
	}
	return st
}

func IsStatusLimit(err error, label string) bool {
	st := status.Convert(err)
	for _, detail := range st.Details() {
		switch v := detail.(type) {
		case *errdetails.QuotaFailure:
			for _, violation := range v.GetViolations() {
				vals := strings.Split(violation.GetDescription(), " ")
				if len(vals) > 1 && vals[1] == label {
					logger.Sugar.Debugf("IsStatusLimit: true: status %s %s", label, st)
					return true
				}
			}
		}
	}
	return false
}
