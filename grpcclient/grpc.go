package grpcclient

import (
	"google.golang.org/grpc"
)

type UnaryClientInterceptor = grpc.UnaryClientInterceptor
type DialOption = grpc.DialOption
type ClientConn = grpc.ClientConn
