package restproxyserver

import (
	"context"
	"fmt"
	"strings"

	grpc_otrace "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	env "github.com/rkvst/go-rkvstcommon/environment"
	"github.com/rkvst/go-rkvstcommon/httpserver"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const MIMEWildcard = runtime.MIMEWildcard

type Marshaler = runtime.Marshaler
type ServeMux = runtime.ServeMux
type DialOption = grpc.DialOption

type RegisterRESTProxyServer func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

type RESTProxyServer struct {
	name        string
	port        string
	log         Logger
	grpcAddress string
	dialOptions []DialOption
	options     []runtime.ServeMuxOption
	register    RegisterRESTProxyServer
	server      *httpserver.Server
}

type RESTProxyServerOption func(*RESTProxyServer)

func WithMarshaler(mime string, m Marshaler) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.options = append(g.options, runtime.WithMarshalerOption(mime, m))
	}
}

// New creates a new RESTProxyServer that is bound to a specific GRPC Gateway API. This object complies with
// the standard Listener interface and can be managed by the startup.Listeners object.
func New(log Logger, name string, r RegisterRESTProxyServer, opts ...RESTProxyServerOption) RESTProxyServer {
	log.Debugf("New RESTPROXY Server %s", name)

	grpcAddress := fmt.Sprintf("localhost:%s", env.GetOrFatal("PORT"))

	restport := env.GetOrFatal("RESTPROXY_PORT")

	g := RESTProxyServer{
		name:        strings.ToLower(name),
		port:        restport,
		grpcAddress: grpcAddress,
		register:    r,
		dialOptions: []DialOption{
			grpc.WithUnaryInterceptor(grpc_otrace.UnaryClientInterceptor()),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
		options: []runtime.ServeMuxOption{},
	}
	g.log = log.WithIndex("restproxyserver", g.String())
	for _, opt := range opts {
		opt(&g)
	}
	log.Debugf("RESTPROXY Server %v", g)

	mux := runtime.NewServeMux(g.options...)

	//err = anchorscheduler.RegisterAnchorSchedulerHandlerFromEndpoint(...)
	err := g.register(context.Background(), mux, grpcAddress, g.dialOptions)
	if err != nil {
		log.Panicf("register error: %w", err)
	}

	g.server = httpserver.New(g.log, g.name, g.port, mux)
	return g
}

func (g *RESTProxyServer) String() string {
	// No logging in this method please.
	return fmt.Sprintf("%s:%s", g.name, g.port)
}

func (g *RESTProxyServer) Listen() error {
	g.log.Infof("Listen")
	return g.server.Listen()
}

func (g *RESTProxyServer) Shutdown(ctx context.Context) error {
	g.log.Infof("Shutdown")
	return g.server.Shutdown(ctx)
}
