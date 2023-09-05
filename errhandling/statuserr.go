package errhandling

import (
	"context"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rkvst/go-rkvstcommon/correlationid"
	"github.com/rkvst/go-rkvstcommon/logger"
)

// so we dont have to import status package everywhere
type Status = status.Status

func NewStatus(c codes.Code, format string, args ...any) *Status {
	return status.Newf(c, format, args...)
}

func StatusError(ctx context.Context, c codes.Code, fmt string, opts ...any) error {
	s := NewStatus(c, fmt, opts...)
	return StatusWithCorrelationIDFromContext(ctx, s).Err()
}

func StatusWithCorrelationIDFromContext(ctx context.Context, s *Status) *Status {
	correlationID := correlationid.FromContext(ctx)
	if correlationID == "" {
		logger.Sugar.Infof("no correlationID in context")
		return s
	}
	logger.Sugar.Debugf("correlationID %s", correlationID)
	st, err := s.WithDetails(&errdetails.RequestInfo{
		RequestId: correlationID,
	})
	if err != nil {
		logger.Sugar.Infof("cannot add correlationID %s: %v", correlationID, err)
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
	logger.Sugar.Debugf("IsStatusLimit: status %s %s", label, st)
	for _, detail := range st.Details() {
		switch v := detail.(type) {
		case *errdetails.QuotaFailure:
			for _, violation := range v.GetViolations() {
				vals := strings.Split(violation.GetDescription(), " ")
				if len(vals) > 1 && vals[1] == label {
					return true
				}
			}
		}
	}
	return false
}
