package grpcclient

import (
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	ErrConflictingOptions = errors.New("Conflicting options")
)

type ClientConn = grpc.ClientConn

type GRPCClient struct {
	name                   string
	log                    Logger
	address                string
	unaryInterceptor       bool
	unaryInterceptorNoAuth bool

	conn *ClientConn
}

// Factory API
type GRPCClientOption func(*GRPCClient)

// UnaryInterceptor is always used unless disabled by this call.
func WithoutUnaryInterceptor() GRPCClientOption {
	return func(t *GRPCClient) {
		t.unaryInterceptor = false
	}
}

// UnaryInterceptorNoAuth is used instead of UnaryInterceptor.
func WithNoAuthUnaryInterceptor() GRPCClientOption {
	return func(t *GRPCClient) {
		t.unaryInterceptorNoAuth = true
	}
}

func NewGRPCClient(log Logger, name string, address string, opts ...GRPCClientOption) GRPCClient {
	t := GRPCClient{
		log:                    log,
		name:                   name,
		address:                address,
		unaryInterceptor:       true,  // default is always present
		unaryInterceptorNoAuth: false, // default is not present
	}
	for _, opt := range opts {
		opt(&t)
	}
	return t
}

// Open - makes the connection using the provided attributes. This is idempotent.
func (g *GRPCClient) Open() error {
	var err error
	var conn *grpc.ClientConn

	// Idempotent
	if g.conn != nil {
		return nil
	}

	if g.address == "" {
		return fmt.Errorf("%s: address is blank", g.name)
	}
	if g.unaryInterceptorNoAuth && g.unaryInterceptor {
		return fmt.Errorf("%s: unaryInterceptor and unaryInterceprorNoAuth are both set: %w", g.name, ErrConflictingOptions)
	}

	// The default interceptors are:
	//
	//      grpc.WithTransportCredentials(insecure.NewCredentials()),
	//      grpc.WithUnaryInterceptor(grpcclient.InternalServiceClientUnaryInterceptor()),
	//
	//  alternatively we can have:
	//
	//      grpc.WithTransportCredentials(insecure.NewCredentials()),
	//      grpc.WithUnaryInterceptor(grpcclient.InternalServiceClientUnaryInterceptorNoAuth()),
	//
	// OR
	//
	//      grpc.WithTransportCredentials(insecure.NewCredentials()),
	//
	g.log.Debugf("Open %s client at %v", g.name, g.address)
	// If the interceptor should be used in every service without exception then add it to
	// the opts list here.
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if g.unaryInterceptorNoAuth {
		g.log.Debugf("Open %s client with UnaryInterceptorNoAuth", g.name)
		opts = append(
			opts,
			grpc.WithUnaryInterceptor(InternalServiceClientUnaryInterceptorNoAuth()),
		)
	}
	if g.unaryInterceptor {
		g.log.Debugf("Open %s client with UnaryInterceptor", g.name)
		opts = append(
			opts,
			grpc.WithUnaryInterceptor(InternalServiceClientUnaryInterceptor()),
		)
	}
	conn, err = grpc.Dial(g.address, opts...)
	if err != nil {
		return err
	}
	g.conn = conn
	g.log.Debugf("Open %s client successful", g.name)
	return nil
}

// Close - should be deferred. This function is idempotent.
func (g *GRPCClient) Close() {
	g.conn = nil
}

func (g *GRPCClient) String() string {
	return g.name
}
func (g *GRPCClient) Connection() *ClientConn {
	return g.conn
}
