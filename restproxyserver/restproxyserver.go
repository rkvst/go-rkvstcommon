package restproxyserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/datatrails/go-datatrails-common/httpserver"
	"github.com/datatrails/go-datatrails-common/tracing"
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

type RegisterServer func(context.Context, *ServeMux, string, []DialOption) error

type HandleChainFunc = httpserver.HandleChainFunc

type filePath struct {
	verb        string
	urlPath     string
	fileHandler func(http.ResponseWriter, *http.Request, map[string]string)
}

// Server represents the grpc-gateway rest serve endpoint.
type Server struct {
	name        string
	port        string
	log         Logger
	grpcAddress string
	grpcHost    string
	dialOptions []DialOption
	options     []runtime.ServeMuxOption
	filePaths   []filePath
	handlers    []HandleChainFunc
	registers   []RegisterServer
	mux         *runtime.ServeMux
	server      *httpserver.Server
}

type ServerOption func(*Server)

// WithMarshaler specifies an optional marshaler.
func WithMarshaler(mime string, m Marshaler) ServerOption {
	return func(g *Server) {
		g.options = append(g.options, runtime.WithMarshalerOption(mime, m))
	}
}

// SetQueryParameterParser adds an intercepror that matches header values.
func SetQueryParameterParser(p QueryParameterParser) ServerOption {
	return func(g *Server) {
		g.options = append(g.options, runtime.SetQueryParameterParser(p))
	}
}

// WithIncomingHeaderMatcher adds an intercepror that matches header values.
func WithIncomingHeaderMatcher(o HeaderMatcherFunc) ServerOption {
	return func(g *Server) {
		if o != nil && !reflect.ValueOf(o).IsNil() {
			g.options = append(g.options, runtime.WithIncomingHeaderMatcher(o))
		}
	}
}

// WithOutgoingHeaderMatcher matches header values on output. Nil argument is ignored.
func WithOutgoingHeaderMatcher(o HeaderMatcherFunc) ServerOption {
	return func(g *Server) {
		if o != nil && !reflect.ValueOf(o).IsNil() {
			g.options = append(g.options, runtime.WithOutgoingHeaderMatcher(o))
		}
	}
}

// WithErrorHandler adds error handling in special cases - e.g on 402 or 429. Nil argument is ignored.
func WithErrorHandler(o ErrorHandlerFunc) ServerOption {
	return func(g *Server) {
		if o != nil && !reflect.ValueOf(o).IsNil() {
			g.options = append(g.options, runtime.WithErrorHandler(o))
		}
	}
}

// WithGRPCAddress - overides the defaultGRPCAddress ('localhost:<port>')
// NB: will be removed
func WithGRPCAddress(a string) ServerOption {
	return func(g *Server) {
		g.grpcAddress = a
	}
}

// WithRegisterHandlers adds grpc-gateway handlers. A nil value will emit an
// error from the Listen() method.
func WithRegisterHandlers(registerers ...RegisterServer) ServerOption {
	return func(g *Server) {
		g.registers = append(g.registers, registerers...)
	}
}

// WithOptionalRegisterHandler adds grpc-gateway handlers. A nil value will be
// ignored.
func WithOptionalRegisterHandlers(registerers ...RegisterServer) ServerOption {
	return func(g *Server) {
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
func WithHTTPHandlers(handlers ...HandleChainFunc) ServerOption {
	return func(g *Server) {
		g.handlers = append(g.handlers, handlers...)
	}
}

// WithOptionalHTTPHandlers adds handlers on the http endpoint. A nil value will
// be ignored.
func WithOptionalHTTPHandlers(handlers ...HandleChainFunc) ServerOption {
	return func(g *Server) {
		for i := 0; i < len(handlers); i++ {
			handler := handlers[i]
			if handler != nil && !reflect.ValueOf(handler).IsNil() {
				g.handlers = append(g.handlers, handler)
			}
		}
	}
}

// WithAppendedDialOption appends a grpc dial option.
func WithAppendedDialOption(d DialOption) ServerOption {
	return func(g *Server) {
		g.dialOptions = append(g.dialOptions, d)
	}
}

// WithPrependedDialOption prepends a grpc dial option.
func WithPrependedDialOption(d DialOption) ServerOption {
	return func(g *Server) {
		g.dialOptions = append([]DialOption{d}, g.dialOptions...)
	}
}

// WithHandlePath add REST file path handler.
func WithHandlePath(verb string, urlPath string, f func(http.ResponseWriter, *http.Request, map[string]string)) ServerOption {
	return func(g *Server) {
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

// New creates a new Server that is bound to a specific GRPC Gateway API. This object complies with
// the standard Listener interface and can be managed by the startup.Listeners object.
func New(log Logger, name string, port string, opts ...ServerOption) *Server {
	var g Server
	return new_(&g, log, name, port, opts...)
}

// function outlining
func new_(g *Server, log Logger, name string, port string, opts ...ServerOption) *Server {

	g.name = strings.ToLower(name)
	g.port = port
	g.dialOptions = tracing.GRPCDialTracingOptions()
	g.options = []runtime.ServeMuxOption{}
	g.filePaths = []filePath{}
	g.handlers = []HandleChainFunc{}
	g.registers = []RegisterServer{}

	for _, opt := range opts {
		opt(g)
	}

	if g.grpcAddress == "" {
		g.grpcAddress = fmt.Sprintf("localhost:%s", port)
	}

	g.mux = runtime.NewServeMux(g.options...)
	g.log = log.WithIndex("restproxyserver", g.name)
	return g
}

func (g *Server) String() string {
	// No logging in this method please.
	return fmt.Sprintf("%s:%s", g.name, g.port)
}

func (g *Server) Listen() error {

	var err error

	for _, p := range g.filePaths {
		err = g.mux.HandlePath(p.verb, p.urlPath, p.fileHandler)
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
		err = register(context.Background(), g.mux, g.grpcAddress, g.dialOptions)
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

func (g *Server) Shutdown(ctx context.Context) error {
	g.log.Infof("Shutdown")
	return g.server.Shutdown(ctx)
}
