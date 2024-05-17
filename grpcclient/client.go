package grpcclient

import (
	"google.golang.org/grpc"
)

type Client struct {
	name    string
	log     Logger
	address string
	conn    *grpc.ClientConn
	options []grpc.DialOption
}

func (g *Client) Open() error {
	if g.conn != nil {
		return nil
	}

	var err error
	var conn *grpc.ClientConn

	g.log.Debugf("Open %s client at %v", g.name, g.address)
	conn, err = grpc.Dial(g.address, g.options...)
	if err != nil {
		return err
	}
	g.conn = conn
	g.log.Debugf("Open %s client successful", g.name)
	return nil
}

func (g *Client) Close() {
	if g.conn != nil {
		g.log.Debugf("Close %s client at %v", g.name, g.address)
		g.conn.Close()
		g.conn = nil
	}
}

func (g *Client) String() string {
	return g.name
}

func (g *Client) Connector() *ClientConn {
	return g.conn
}

type ClientOption func(*Client)

func WithDialOptions(d ...DialOption) ClientOption {
	return func(t *Client) {
		t.options = append(t.options, d...)
	}
}

func New(log Logger, name string, address string, opts ...ClientOption) *Client {
	t := Client{
		name:    name,
		address: address,
		log:     log.WithIndex("grpcclient", name),
		options: []grpc.DialOption{},
	}
	for _, opt := range opts {
		opt(&t)
	}
	return &t
}
