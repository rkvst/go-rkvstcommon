package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// A http server that has an inbuilt logger, name and complies wuth the Listener interface in
// startup.Listeners.

type HTTPServer struct {
	http.Server
	log  Logger
	name string
}

func NewHTTPServer(log Logger, name string, port string, handler http.Handler) *HTTPServer {
	m := HTTPServer{
		Server: http.Server{
			Addr:    ":" + port,
			Handler: handler,
		},
		name: name,
	}
	m.log = log.WithIndex("httpserver", m.String())
	// It is preferable to return a copy rather than a reference. Unfortunately http.Server has an
	// internal mutex and this cannot or should not be copied so we will return a reference instead.
	return &m
}

func (m *HTTPServer) String() string {
	// No logging here please
	return fmt.Sprintf("%s%s", m.name, m.Addr)
}

func (m *HTTPServer) Listen() error {
	m.log.Infof("httpserver starting")
	err := m.ListenAndServe()
	if err != nil {
		return fmt.Errorf("%s server terminated: %v", m, err)
	}
	return nil
}

func (m *HTTPServer) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m.log.Infof("httpserver shutdown")
	return m.Server.Shutdown(ctx)
}
