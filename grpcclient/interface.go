package grpcclient

type ClientProvider interface {
	Open() error
	Close()
	String() string
}
