package restproxyserver

import (
	"context"
	"fmt"

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
	log         Logger
	grpcAddress   string
	dialOptions []DialOption
	options     []runtime.ServeMuxOption
	register    RegisterRESTProxyServer
	server      *httpserver.HTTPServer
}

type RESTProxyServerOption func(*RESTProxyServer)

func WithMarshaler(mime string, m Marshaler) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.options = append(g.options, runtime.WithMarshalerOption(mime, m))
	}
}

// NewRESTProxyServer cretaes a new RESTProxyServer that is bound to a specific GRPC Gateway API. This object complies with
// the standard Listener service and can be managed by the startup.Listeners object.
func New(log Logger, name string, r RegisterRESTProxyServer, opts ...RESTProxyServerOption) RESTProxyServer {
	port := env.GetOrFatal("RESTPROXY_PORT")
	grpcAddress := fmt.Sprintf("localhost:%s", port)

	g := RESTProxyServer{
		name:      name,
		grpcAddress: grpcAddress,
		register:  r,
		dialOptions: []DialOption{
			grpc.WithUnaryInterceptor(grpc_otrace.UnaryClientInterceptor()),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
		options: []runtime.ServeMuxOption{},
	}
	for _, opt := range opts {
		opt(&g)
	}

	//err = anchorscheduler.RegisterAnchorSchedulerHandlerFromEndpoint(...)
	mux := runtime.NewServeMux(g.options...)
	err := g.register(context.Background(), mux, grpcAddress, g.dialOptions)
	if err != nil {
		log.Panicf("register error: %w", err)
	}

	g.log = log.WithIndex("restproxyserver", g.String())

	g.server = httpserver.NewHTTPServer(g.log, name, port, mux)
	return g
}

func (g *RESTProxyServer) String() string {
	// No logging in this method please.
	return fmt.Sprintf("%s%s", g.name, g.grpcAddress)
}

func (g *RESTProxyServer) Listen() error {
	g.log.Infof("Listen")
	return g.server.Listen()
}

func (g *RESTProxyServer) Shutdown(ctx context.Context) error {
	g.log.Infof("Shutdown")
	return g.server.Shutdown(ctx)
}
