package grpcserver

import (
	"context"
	"strings"

	"google.golang.org/grpc"

	"github.com/rkvst/go-rkvstcommon/correlationid"
	"github.com/rkvst/go-rkvstcommon/logger"
)

const (
	archivistPrefix = "/archivist"
)

// CorrelationIDUnaryServerInterceptor returns a new unary server interceptor that inserts correlationID into context
func CorrelationIDUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {

		// only for archivist endpoint and not /health or /metrics.
		// - without this some services refused to become ready (locations and all creators)
		logger.Sugar.Debugf("info.FullMethod: %s", info.FullMethod)
		if !strings.HasPrefix(info.FullMethod, archivistPrefix) {
			return handler(ctx, req)
		}

		ctx = correlationid.Context(ctx)
		logger.Sugar.Debugf("correlationID: %v", correlationid.FromContext(ctx))
		return handler(ctx, req)
	}
}
