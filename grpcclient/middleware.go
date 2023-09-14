package grpcclient

import (
	"context"
	"fmt"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_otrace "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/rkvst/go-rkvstcommon/logger"
	"github.com/rkvst/go-rkvstcommon/tracing"
)

// RFC 6648 should apply. Section 3.3 forbids the use of field names starting with 'x-'
// TODO: remove x- and use archivist throughout...
const (
	// ArchivistInternalMetadataKey is set for internal calls, but not for edge calls
	ArchivistInternalMetadataKey = "x-archivist-internal"
	// ArchivistInternalMetadataValue is the value to set / check
	ArchivistInternalMetadataValue = "true"

	AuthorizationHeaderKey = "authorization"
)

// propagateMetadataClientUnaryInterceptor passes all metadata from the
// incoming metadata to the outgoing metadata for each downstream request.
func propagateMetadataClientUnaryInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.Sugar.Errorf("Unable to get metadata from context")
		return fmt.Errorf("could not get incoming metadata (for metadata forwarding)")
	}
	// Should not be modified, may cause races
	// https://github.com/grpc/grpc-go/blob/89faf1c3e8283dd3c863b877bcf1631d1fe6f50c/metadata/metadata.go#L166
	md = md.Copy()

	// For the moment we pass everything though, and add the 'internal' key
	// We may decide to strip any content that does not have the prefix
	// ArchivistInternalMetadataPrefix at some point
	md.Set(ArchivistInternalMetadataKey, ArchivistInternalMetadataValue)

	traceID := md.Get(tracing.TraceID)
	if len(traceID) > 0 {
		logger.Sugar.Debugf("%s '%v'", tracing.TraceID, traceID)
	}
	// We use NewOutgoingContext() to make sure we have a clean, empty outgoing context
	// for each call as we don't want any uncontrolled content coming in from the context.
	// Other chained interceptors can be added, if so they must use
	// metadata.AppendToOutgoingContext() to add extra content.
	newCtx := metadata.NewOutgoingContext(ctx, md)

	return invoker(newCtx, method, req, reply, cc, opts...)
}

// stripAuthClientUnaryInterceptor strips authentication metadata from
// outgoing metadata for each downstream request.
// XXX: It should go away when we implement User Story 3010
func stripAuthClientUnaryInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		logger.Sugar.Errorf("Unable to get metadata from context")
		return fmt.Errorf("could not get outgoing metadata (for auth striping)")
	}
	// Should not be modified, may cause races
	// https://github.com/grpc/grpc-go/blob/89faf1c3e8283dd3c863b877bcf1631d1fe6f50c/metadata/metadata.go#L166
	md = md.Copy()

	// remove the authorization (authentication) headers
	// we leave the rest (including the tenant ID, intact)
	delete(md, AuthorizationHeaderKey)

	// We use NewOutgoingContext() to make sure we have a clean, empty outgoing context
	// for each call as we don't want any uncontrolled content coming in from the context.
	// Other chained interceptors can be added, if so they must use
	// metadata.AppendToOutgoingContext() to add extra content.
	newCtx := metadata.NewOutgoingContext(ctx, md)

	return invoker(newCtx, method, req, reply, cc, opts...)
}

// InternalServiceClientUnaryInterceptor returns a client interceptor that
// pass all metadata from the incoming metadata to the outgoing metadata for
// each downstream request and adds open tracing span and headers.
// This should be used as a grpc Dial option for _all_ internal services and
// not for _any_ external services
func InternalServiceClientUnaryInterceptor() grpc.UnaryClientInterceptor {
	return grpc_middleware.ChainUnaryClient(
		propagateMetadataClientUnaryInterceptor,
		grpc_otrace.UnaryClientInterceptor(),
	)
}

// InternalServiceClientUnaryInterceptorNoAuth returns a client interceptor that
// pass some metadata from the incoming metadata to the outgoing metadata for
// each downstream request and adds open tracing span and headers.
// It does NOT include the authorisation metadata, but it DOES include all the other
// standard contents.
//
// This should ONLY be used as a grpc Dial option for internal sercices that need
// to make calls that skip PDP (ABAC) authorisation.
// XXX: It should go away when we implement User Story 3010
func InternalServiceClientUnaryInterceptorNoAuth() grpc.UnaryClientInterceptor {
	return grpc_middleware.ChainUnaryClient(
		InternalServiceClientUnaryInterceptor(),
		stripAuthClientUnaryInterceptor,
	)
}
