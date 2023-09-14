package middleware

import (
	"context"
	"strings"

	"github.com/rkvst/go-rkvstcommon/correlationid"
	"github.com/rkvst/go-rkvstcommon/logger"
	"google.golang.org/grpc"
)

// Manifest constants for context metadata that need to be co-ordinated platform
// wide. And which don't more logically belong in a specific go-rkvstcommon
// package. Most of these originated in avid/src/wal
//
// Good candidates are constants already needed in many services.

// RFC 6648 should apply. Section 3.3 forbids the use of field names starting with 'x-'
// TODO: remove x- and use archivist throughout...
const (
	// ArchivistInternalMetadataPrefix should be used for all context metadata
	// anything with this prefix should be stripped from incoming requets
	// but is passed through to downstream services
	ArchivistInternalMetadataPrefix = "x-archivist-internal-"

	ArchivistInternalMetadataKey = "x-archivist-internal"
	// ArchivistInternalMetadataValue is the value to set / check
	ArchivistInternalMetadataValue = "true"

	ArchivistPrefix = "/archivist"
)

// CorrelationIDUnaryServerInterceptor returns a new unary server interceptor that inserts correlationID into context
func CorrelationIDUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {

		// only for archivist endpoint and not /health or /metrics.
		// - without this some services refused to become ready (locations and all creators)
		logger.Sugar.Debugf("info.FullMethod: %s", info.FullMethod)
		if !strings.HasPrefix(info.FullMethod, ArchivistPrefix) {
			return handler(ctx, req)
		}

		ctx = correlationid.Context(ctx)
		logger.Sugar.Debugf("correlationID: %v", correlationid.FromContext(ctx))
		return handler(ctx, req)
	}
}
