package restproxyserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	env "github.com/rkvst/go-rkvstcommon/environment"
	"github.com/rkvst/go-rkvstcommon/httpserver"
	"github.com/rkvst/go-rkvstcommon/tracing"

	"google.golang.org/grpc"
)

const (
	defaultGRPCHost = "localhost"
	MIMEWildcard    = runtime.MIMEWildcard
)

type Marshaler = runtime.Marshaler
type ServeMux = runtime.ServeMux
type QueryParameterParser = runtime.QueryParameterParser
type DialOption = grpc.DialOption

type RegisterRESTProxyServer func(context.Context, *ServeMux, string, []DialOption) error

type HandleFunc func(http.Handler) http.Handler

type filePath struct {
	verb        string
	urlPath     string
	fileHandler func(http.ResponseWriter, *http.Request, map[string]string)
}

// RESTProxyServer represents the grpc-gateway rest openapiv2 serve endpoint.
type RESTProxyServer struct {
	name        string
	port        string
	log         Logger
	grpcAddress string
	grpcHost    string
	dialOptions []DialOption
	options     []runtime.ServeMuxOption
	filePaths   []filePath
	handlers    []HandleFunc
	register    RegisterRESTProxyServer
	health      RegisterRESTProxyServer
	server      *httpserver.Server
}

type RESTProxyServerOption func(*RESTProxyServer)

// WithMarshaler specifies on an optional marshaler.
func WithMarshaler(mime string, m Marshaler) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.options = append(g.options, runtime.WithMarshalerOption(mime, m))
	}
}

// SetQueryParameterParser adds an intercepror that matches header values.
func SetQueryParameterParser(p runtime.QueryParameterParser) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.options = append(g.options, runtime.SetQueryParameterParser(p))
	}
}

// WithOutgoingHeaderMatcher matches header values on oupput.
// WithIncomingHeaderMatcher adds an intercepror that matches header values.
func WithIncomingHeaderMatcher(o runtime.HeaderMatcherFunc) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.options = append(g.options, runtime.WithIncomingHeaderMatcher(o))
	}
}

// WithOutgoingHeaderMatcher matches header values on oupput.
func WithOutgoingHeaderMatcher(o runtime.HeaderMatcherFunc) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.options = append(g.options, runtime.WithOutgoingHeaderMatcher(o))
	}
}

// WithErrorHandler adds error handling in special cases - e.g on 402 or 429.
func WithErrorHandler(o runtime.ErrorHandlerFunc) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.options = append(g.options, runtime.WithErrorHandler(o))
	}
}

// WithGRPCAddress - overides the defaultGRPSAddress ('localhost:<port>')
func WithGRPCAddress(a string) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.grpcAddress = a
	}
}

// WikthHealthHandler adds another grpc-gateway - typically grpcHealth.
func WithHealthHandler(r RegisterRESTProxyServer) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.health = r
	}
}

// WithHandler adds a handler on the http endpoint.
func WithHandler(h HandleFunc) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		if h != nil {
			g.handlers = append(g.handlers, h)
		}
	}
}

// WithAppendedDialOption appends a grpc dial option.
func WithAppendedDialOption(d DialOption) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.dialOptions = append(g.dialOptions, d)
	}
}

// WithPrependedDialOption prepends a grpc dial option.
func WithPrependedDialOption(d DialOption) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.dialOptions = append([]DialOption{d}, g.dialOptions...)
	}
}

// WithHandlePath add REST file path handler.
func WithHandlePath(verb string, urlPath string, f func(http.ResponseWriter, *http.Request, map[string]string)) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.filePaths = append(
			g.filePaths,
			filePath{
				verb:        verb,
				urlPath:     urlPath,
				fileHandler: f,
			},
		)
	}
}

// New creates a new RESTProxyServer that is bound to a specific GRPC Gateway API. This object complies with
// the standard Listener interface and can be managed by the startup.Listeners object.
func New(log Logger, name string, port string, r RegisterRESTProxyServer, opts ...RESTProxyServerOption) RESTProxyServer {
	var err error

	log.Debugf("New RESTPROXY Server %s", name)

	g := RESTProxyServer{
		name:        strings.ToLower(name),
		port:        port,
		register:    r,
		dialOptions: tracing.GRPCDialTracingOptions(),
		options:     []runtime.ServeMuxOption{},
		filePaths:   []filePath{},
		handlers:    []HandleFunc{},
	}
	g.log = log.WithIndex("restproxyserver", g.String())
	for _, opt := range opts {
		opt(&g)
	}

	if g.grpcAddress == "" {
		port := env.GetOrFatal("PORT")
		g.grpcAddress = fmt.Sprintf("localhost:%s", port)
	}

	log.Debugf("RESTPROXY Server")

	mux := runtime.NewServeMux(g.options...)
	for _, p := range g.filePaths {
		err = mux.HandlePath(p.verb, p.urlPath, p.fileHandler)
		if err != nil {
			log.Panicf("cannot handle path %s: %w", p.urlPath, err)
		}
	}

	err = g.register(context.Background(), mux, g.grpcAddress, g.dialOptions)
	if err != nil {
		log.Panicf("register error: %w", err)
	}
	if g.health != nil {
		err = g.health(context.Background(), mux, g.grpcAddress, g.dialOptions)
		if err != nil {
			log.Panicf("healthregister error: %w", err)
		}
	}

	var h http.Handler = mux
	for _, handler := range g.handlers {
		h = handler(h)
	}
	g.server = httpserver.New(g.log, fmt.Sprintf("proxy %s", g.name), g.port, h)
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
