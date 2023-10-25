package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	//grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_otrace "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	env "github.com/rkvst/go-rkvstcommon/environment"
	"github.com/rkvst/go-rkvstcommon/grpchealth"
	grpcHealth "google.golang.org/grpc/health/grpc_health_v1"
)

// so we dont have to import grpc when using this package.
type grpcServer = grpc.Server
type grpcUnaryServerInterceptor = grpc.UnaryServerInterceptor

type RegisterServer func(*grpcServer)

func defaultRegisterServer(g *grpcServer) {}

type GRPCServer struct {
	name         string
	log          Logger
	listenStr    string
	health       *grpchealth.HealthCheckingService
	interceptors []grpcUnaryServerInterceptor
	register     RegisterServer
	server       *grpcServer
}

type GRPCServerOption func(*GRPCServer)

func WithAppendedInterceptor(i grpcUnaryServerInterceptor) GRPCServerOption {
	return func(g *GRPCServer) {
		g.interceptors = append(g.interceptors, i)
	}
}

func WithPrependedInterceptor(i grpcUnaryServerInterceptor) GRPCServerOption {
	return func(g *GRPCServer) {
		g.interceptors = append([]grpcUnaryServerInterceptor{i}, g.interceptors...)
	}
}

func WithRegisterServer(r RegisterServer) GRPCServerOption {
	return func(g *GRPCServer) {
		g.register = r
	}
}

func tracingFilter(ctx context.Context, fullMethodName string) bool {
	if fullMethodName == grpcHealth.Health_Check_FullMethodName {
		return false
	}
	return true
}

// New cretaes a new GRPCServer that is bound to a specific GRPC API. This object complies with
// the standard Listener service and can be managed by the startup.Listeners object.
func New(log Logger, name string, opts ...GRPCServerOption) GRPCServer {
	listenStr := fmt.Sprintf(":%s", env.GetOrFatal("PORT"))

	health := grpchealth.New(log)

	g := GRPCServer{
		name:      strings.ToLower(name),
		listenStr: listenStr,
		health:    &health,
		register:  defaultRegisterServer,
		interceptors: []grpc.UnaryServerInterceptor{
			grpc_otrace.UnaryServerInterceptor(grpc_otrace.WithFilterFunc(tracingFilter)),
			grpc_validator.UnaryServerInterceptor(),
		},
	}
	for _, opt := range opts {
		opt(&g)
	}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(g.interceptors...),
		),
	)

	// RegisterAccessPoliciesServer(s grpc.ServiceRegistrar, srv AccessPoliciesServer)
	//accessPolicyV1API.RegisterAccessPoliciesServer(server, s)
	g.register(server)
	grpcHealth.RegisterHealthServer(server, &health)
	reflection.Register(server)

	g.server = server
	g.log = log.WithIndex("grpcserver", g.String())
	return g
}

func (g *GRPCServer) String() string {
	// No logging in this method please.
	return fmt.Sprintf("%s%s", g.name, g.listenStr)
}

func (g *GRPCServer) Listen() error {
	listen, err := net.Listen("tcp", g.listenStr)
	if err != nil {
		return fmt.Errorf("failed to listen %s: %w", g, err)
	}

	g.health.Ready() // readiness

	g.log.Infof("Listen")
	err = g.server.Serve(listen)
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("failed to serve %s: %w", g, err)
	}
	return nil
}

func (g *GRPCServer) Shutdown(_ context.Context) error {
	g.log.Infof("Shutdown")
	g.health.NotReady() // readiness
	g.health.Dead()     // liveness
	g.server.GracefulStop()
	return nil
}
