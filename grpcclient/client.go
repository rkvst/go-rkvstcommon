package grpcclient

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ClientConn = grpc.ClientConn

type GRPCClient struct {
	name                   string
	log                    Logger
	address                string
	noUnaryInterceptor     bool
	noAuthUnaryInterceptor bool

	conn *ClientConn
}

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
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if g.noAuthUnaryInterceptor {
		g.log.Debugf("Open %s client with UnaryInterceptorNoAuth", g.name)
		opts = append(
			opts,
			grpc.WithUnaryInterceptor(InternalServiceClientUnaryInterceptorNoAuth()),
		)
	} else if !g.noUnaryInterceptor {
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

// Close - should be deferred
func (g *GRPCClient) Close() {
	g.conn = nil
}

func (g *GRPCClient) String() string {
	return g.name
}
func (g *GRPCClient) Connection() *ClientConn {
	return g.conn
}

type GRPCClientOption func(*GRPCClient)

func WithoutUnaryInterceptor() GRPCClientOption {
	return func(t *GRPCClient) {
		t.noUnaryInterceptor = true
	}
}

// Takes precedence over WithoutUnaryInterceptor
func WithNoAuthUnaryInterceptor() GRPCClientOption {
	return func(t *GRPCClient) {
		t.noAuthUnaryInterceptor = true
	}
}

func NewGRPCClient(log Logger, name string, address string, opts ...GRPCClientOption) GRPCClient {
	t := GRPCClient{
		log:     log,
		name:    name,
		address: address,
	}
	for _, opt := range opts {
		opt(&t)
	}
	return t
}
