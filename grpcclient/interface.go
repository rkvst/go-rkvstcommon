package grpcclient

type GRPCClientProvider interface {
	Open() error
	Close()
	String() string
	Connection() *ClientConn
}
