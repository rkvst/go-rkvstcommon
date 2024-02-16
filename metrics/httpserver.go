package metrics

import (
	"fmt"

	"github.com/datatrails/go-datatrails-common/httpserver"
)

type HTTPServer struct {
	*httpserver.Server
	metrics *Metrics
}

func NewServer(log Logger, serviceName string, port string, opts ...MetricsOption) HTTPServer {
	m := New(
		log,
		serviceName,
		port,
	)
	return HTTPServer{
		httpserver.New(log, fmt.Sprintf("metrics %s", serviceName), port, m.newPromHandler()),
		m,
	}
}

func (h *HTTPServer) Metrics() *Metrics {
	return h.metrics
}
