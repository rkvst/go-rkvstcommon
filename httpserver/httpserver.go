package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type HandleChainFunc func(http.Handler) http.Handler

// A http server that has an inbuilt logger, name and complies wuth the Listener interface in
// startup.Listeners.

type Server struct {
	log      Logger
	name     string
	server   http.Server
	handlers []HandleChainFunc
}

type ServerOption func(*Server)

// WithHandler adds a handler on the http endpoint.
func WithHandler(h HandleChainFunc) ServerOption {
	return func(s *Server) {
		if h != nil {
			s.handlers = append(s.handlers, h)
		}
	}
}

// WithHandlers adds a handler on the http endpoint.
func WithHandlers(h []HandleChainFunc) ServerOption {
	return func(s *Server) {
		s.handlers = append(s.handlers, h...)
	}
}

func New(log Logger, name string, port string, h http.Handler, opts ...ServerOption) *Server {
	s := Server{
		server: http.Server{
			Addr: ":" + port,
		},
		name: strings.ToLower(name),
	}
	s.log = log.WithIndex("httpserver", s.String())
	for _, opt := range opts {
		opt(&s)
	}

	s.log.Debugf("Initialise handlers %v", h)
	for _, handler := range s.handlers {
		if handler != nil {
			h = handler(h)
		}
	}
	s.server.Handler = h

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
