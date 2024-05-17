package restproxyserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	env "github.com/datatrails/go-datatrails-common/environment"
	"github.com/datatrails/go-datatrails-common/httpserver"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"google.golang.org/grpc"
)

const (
	defaultGRPCHost = "localhost"
	MIMEWildcard    = runtime.MIMEWildcard
)

var (
	ErrNilRegisterer      = errors.New("Nil Registerer")
	ErrNilRegistererValue = errors.New("Nil Registerer value")
)

type Marshaler = runtime.Marshaler
type ServeMux = runtime.ServeMux
type QueryParameterParser = runtime.QueryParameterParser
type HeaderMatcherFunc = runtime.HeaderMatcherFunc
type ErrorHandlerFunc = runtime.ErrorHandlerFunc
type DialOption = grpc.DialOption

type RegisterRESTProxyServer func(context.Context, *ServeMux, string, []DialOption) error

type HandleChainFunc = httpserver.HandleChainFunc

type filePath struct {
	verb        string
	urlPath     string
	fileHandler func(http.ResponseWriter, *http.Request, map[string]string)
}

// RESTProxyServer represents the grpc-gateway rest serve endpoint.
type RESTProxyServer struct {
	name        string
	port        string
	log         Logger
	grpcAddress string
	grpcHost    string
	dialOptions []DialOption
	options     []runtime.ServeMuxOption
	filePaths   []filePath
	handlers    []HandleChainFunc
	registers   []RegisterRESTProxyServer
	mux         *runtime.ServeMux
	server      *httpserver.Server
}

type RESTProxyServerOption func(*RESTProxyServer)

// WithMarshaler specifies an optional marshaler.
func WithMarshaler(mime string, m Marshaler) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.options = append(g.options, runtime.WithMarshalerOption(mime, m))
	}
}

// SetQueryParameterParser adds an intercepror that matches header values.
func SetQueryParameterParser(p QueryParameterParser) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.options = append(g.options, runtime.SetQueryParameterParser(p))
	}
}

// WithIncomingHeaderMatcher adds an intercepror that matches header values.
func WithIncomingHeaderMatcher(o HeaderMatcherFunc) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		if o != nil && !reflect.ValueOf(o).IsNil() {
			g.options = append(g.options, runtime.WithIncomingHeaderMatcher(o))
		}
	}
}

// WithOutgoingHeaderMatcher matches header values on output. Nil argument is ignored.
func WithOutgoingHeaderMatcher(o HeaderMatcherFunc) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		if o != nil && !reflect.ValueOf(o).IsNil() {
			g.options = append(g.options, runtime.WithOutgoingHeaderMatcher(o))
		}
	}
}

// WithErrorHandler adds error handling in special cases - e.g on 402 or 429. Nil argument is ignored.
func WithErrorHandler(o ErrorHandlerFunc) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		if o != nil && !reflect.ValueOf(o).IsNil() {
			g.options = append(g.options, runtime.WithErrorHandler(o))
		}
	}
}

// WithGRPCAddress - overides the defaultGRPCAddress ('localhost:<port>')
func WithGRPCAddress(a string) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.grpcAddress = a
	}
}

// WithRegisterHandlers adds grpc-gateway handlers. A nil value will emit an
// error from the Listen() method.
func WithRegisterHandlers(registerers ...RegisterRESTProxyServer) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.registers = append(g.registers, registerers...)
	}
}

// WithOptionalRegisterHandler adds grpc-gateway handlers. A nil value will be
// ignored.
func WithOptionalRegisterHandlers(registerers ...RegisterRESTProxyServer) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		for i := 0; i < len(registerers); i++ {
			registerer := registerers[i]
			if registerer != nil && !reflect.ValueOf(registerer).IsNil() {
				g.registers = append(g.registers, registerer)
			}
		}
	}
}

// WithHTTPHandlers adds handlers on the http endpoint. A nil value will
// return an error on executiong Listen()
func WithHTTPHandlers(handlers ...HandleChainFunc) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.handlers = append(g.handlers, handlers...)
	}
}

// WithOptionalHTTPHandlers adds handlers on the http endpoint. A nil value will
// be ignored.
func WithOptionalHTTPHandlers(handlers ...HandleChainFunc) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		for i := 0; i < len(handlers); i++ {
			handler := handlers[i]
			if handler != nil && !reflect.ValueOf(handler).IsNil() {
				g.handlers = append(g.handlers, handler)
			}
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
func WithPrependedDialOptions(d ...DialOption) RESTProxyServerOption {
	return func(g *RESTProxyServer) {
		g.dialOptions = append(d, g.dialOptions...)
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
func New(log Logger, name string, port string, opts ...RESTProxyServerOption) RESTProxyServer {

	g := RESTProxyServer{
		name:        strings.ToLower(name),
		port:        port,
		dialOptions: []grpc.DialOption{},
		options:     []runtime.ServeMuxOption{},
		filePaths:   []filePath{},
		handlers:    []HandleChainFunc{},
		registers:   []RegisterRESTProxyServer{},
	}
	g.log = log.WithIndex("restproxyserver", g.String())
	for _, opt := range opts {
		opt(&g)
	}

	if g.grpcAddress == "" {
		port := env.GetOrFatal("PORT")
		g.grpcAddress = fmt.Sprintf("localhost:%s", port)
	}

	g.mux = runtime.NewServeMux(g.options...)
	return g
}

func (g *RESTProxyServer) String() string {
	// No logging in this method please.
	return fmt.Sprintf("%s:%s", g.name, g.port)
}

func (g *RESTProxyServer) Listen() error {

	for _, p := range g.filePaths {
		err := g.mux.HandlePath(p.verb, p.urlPath, p.fileHandler)
		if err != nil {
			return fmt.Errorf("cannot handle path %s: %w", p.urlPath, err)
		}
	}

	for _, register := range g.registers {
		if register == nil {
			return ErrNilRegisterer
		}
		if reflect.ValueOf(register).IsNil() {
			return ErrNilRegistererValue
		}
		err := register(context.Background(), g.mux, g.grpcAddress, g.dialOptions)
		if err != nil {
			return err
		}
	}
	g.server = httpserver.New(
		g.log,
		fmt.Sprintf("proxy %s", g.name),
		g.port,
		g.mux,
		httpserver.WithHandlers(g.handlers...),
	)

	g.log.Debugf("server %v", g.server)
	g.log.Infof("Listen")
	return g.server.Listen()
}

func (g *RESTProxyServer) Shutdown(ctx context.Context) error {
	g.log.Infof("Shutdown")
	return g.server.Shutdown(ctx)
}
