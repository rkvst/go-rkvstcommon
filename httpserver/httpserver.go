package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"
)

var (
	ErrNilHandler            = errors.New("Nil Handler")
	ErrNilHandlerValue       = errors.New("Nil Handler value")
	ErrHandlerFuncReturnsNil = errors.New("Handler function returns nil")
)

type HandleChainFunc func(http.Handler) http.Handler

// A http server that has an inbuilt logger, name and complies wuth the Listener interface in
// startup.Listeners.

type Server struct {
	log      Logger
	name     string
	server   http.Server
	handler  http.Handler
	handlers []HandleChainFunc
}

type ServerOption func(*Server)

// WithHandlers adds a handler on the http endpoint. If the handler is nil
// then an error will occur when executing the Listen() method.
func WithHandlers(handlers ...HandleChainFunc) ServerOption {
	return func(s *Server) {
		s.handlers = append(s.handlers, handlers...)
	}
}

// WithOptionalHandlers adds a handler on the http endpoint. If the handler is nil
// it is ignored.
func WithOptionalHandlers(handlers ...HandleChainFunc) ServerOption {
	return func(s *Server) {
		for i := 0; i < len(handlers); i++ {
			handler := handlers[i]
			if handler != nil && !reflect.ValueOf(handler).IsNil() {
				s.handlers = append(s.handlers, handler)
			}
		}
	}
}

// New creates a new httpserver. ChainHandlers that return nil are ignored.
func New(log Logger, name string, port string, h http.Handler, opts ...ServerOption) *Server {
	s := Server{
		server: http.Server{
			Addr: ":" + port,
		},
		handler: h,
		name:    strings.ToLower(name),
	}
	s.log = log.WithIndex("httpserver", s.String())
	for _, opt := range opts {
		opt(&s)
	}
	// It is preferable to return a copy rather than a reference. Unfortunately http.Server has an
	// internal mutex and this cannot or should not be copied so we will return a reference instead.
	return &s
}

func (s *Server) String() string {
	// No logging here please
	return fmt.Sprintf("%s%s", s.name, s.server.Addr)
}

func (s *Server) Listen() error {
	s.log.Infof("Listen")
	h := s.handler
	for i, handler := range s.handlers {
		s.log.Debugf("%d: h %v handler %v", i, handler)
		if handler == nil {
			return ErrNilHandler
		}
		if reflect.ValueOf(handler).IsNil() {
			return ErrNilHandlerValue
		}
		h1 := handler(h)
		if h1 == nil {
			return ErrHandlerFuncReturnsNil
		}
		h = h1
	}
	s.server.Handler = h

	// this is a blocking operation
	err := s.server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("%s server terminated: %v", s, err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	s.log.Infof("Shutdown")
	err := s.server.Shutdown(ctx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
