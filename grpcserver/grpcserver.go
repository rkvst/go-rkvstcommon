package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_otrace "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/datatrails/go-datatrails-common/grpchealth"
	grpcHealth "google.golang.org/grpc/health/grpc_health_v1"
)

// so we dont have to import grpc when using this package.
type grpcServer = grpc.Server
type grpcUnaryServerInterceptor = grpc.UnaryServerInterceptor

type RegisterServer func(*grpcServer)

func defaultRegisterServer(g *grpcServer) {}

type Server struct {
	name          string
	log           Logger
	listenStr     string
	health        bool
	healthService *grpchealth.HealthCheckingService
	interceptors  []grpcUnaryServerInterceptor
	register      RegisterServer
	server        *grpcServer
	reflection    bool
}

type ServerOption func(*Server)

func WithAppendedInterceptor(i grpcUnaryServerInterceptor) ServerOption {
	return func(g *Server) {
		g.interceptors = append(g.interceptors, i)
	}
}

func WithPrependedInterceptor(i grpcUnaryServerInterceptor) ServerOption {
	return func(g *Server) {
		g.interceptors = append([]grpcUnaryServerInterceptor{i}, g.interceptors...)
	}
}

func WithRegisterServer(r RegisterServer) ServerOption {
	return func(g *Server) {
		g.register = r
	}
}

func WithoutHealth() ServerOption {
	return func(g *Server) {
		g.health = false
	}
}

func WithReflection(r bool) ServerOption {
	return func(g *Server) {
		g.reflection = r
	}
}

func tracingFilter(ctx context.Context, fullMethodName string) bool {
	if fullMethodName == grpcHealth.Health_Check_FullMethodName {
		return false
	}
	return true
}

// New creates a new Server that is bound to a specific GRPC API. This object complies with
// the standard Listener service and can be managed by the startup.Listeners object.
func New(log Logger, name string, port string, opts ...ServerOption) *Server {
	var g Server

	listenStr := fmt.Sprintf(":%s", port)

	g.name = strings.ToLower(name)
	g.listenStr = listenStr
	g.register = defaultRegisterServer
	g.interceptors = []grpc.UnaryServerInterceptor{
		grpc_otrace.UnaryServerInterceptor(grpc_otrace.WithFilterFunc(tracingFilter)),
		grpc_validator.UnaryServerInterceptor(),
	}
	g.health = true
	for _, opt := range opts {
		opt(&g)
	}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(g.interceptors...),
		),
	)

	g.register(server)

	if g.health {
		healthService := grpchealth.New(log)
		g.healthService = &healthService
		grpcHealth.RegisterHealthServer(server, g.healthService)
	}

	if g.reflection {
		reflection.Register(server)
	}

	g.server = server
	g.log = log.WithIndex("grpcserver", g.String())
	return &g
}

func (g *Server) String() string {
	// No logging in this method please.
	return fmt.Sprintf("%s%s", g.name, g.listenStr)
}

func (g *Server) Listen() error {
	listen, err := net.Listen("tcp", g.listenStr)
	if err != nil {
		return fmt.Errorf("failed to listen %s: %w", g, err)
	}

	if g.healthService != nil {
		g.healthService.Ready() // readiness
	}

	g.log.Infof("Listen")
	err = g.server.Serve(listen)
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("failed to serve %s: %w", g, err)
	}
	return nil
}

func (g *Server) Shutdown(_ context.Context) error {
	g.log.Infof("Shutdown")
	if g.healthService != nil {
		g.healthService.NotReady() // readiness
		g.healthService.Dead()     // liveness
	}
	g.server.GracefulStop()
	return nil
}
