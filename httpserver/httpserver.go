package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// A http server that has an inbuilt logger, name and complies wuth the Listener interface in
// startup.Listeners.

type Server struct {
	http.Server
	log  Logger
	name string
}

func New(log Logger, name string, port string, handler http.Handler) *Server {
	log.Debugf("New HTTPServer %s", name)
	m := Server{
		Server: http.Server{
			Addr:    ":" + port,
			Handler: handler,
		},
		name: strings.ToLower(name),
	}
	m.log = log.WithIndex("httpserver", m.String())
	// It is preferable to return a copy rather than a reference. Unfortunately http.Server has an
	// internal mutex and this cannot or should not be copied so we will return a reference instead.
	log.Debugf("HTTPServer")
	return &m
}

func (m *Server) String() string {
	// No logging here please
	return fmt.Sprintf("%s%s", m.name, m.Addr)
}

func (m *Server) Listen() error {
	m.log.Infof("Listen")
	err := m.Server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("%s server terminated: %v", m, err)
	}
	return nil
}

func (m *Server) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m.log.Infof("Shutdown")
	err := m.Server.Shutdown(ctx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
