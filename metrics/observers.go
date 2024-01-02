package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type Prometheus interface {
}

type latencyObserveOffset struct {
	label  string
	offset int
}

// Latency observers
type LatencyObservers struct {
	requestsCounter *prometheus.CounterVec
	requestsLatency *prometheus.HistogramVec
	serviceName     string
	labels          []latencyObserveOffset
	log             Logger
}

// NewLatencyObservers is specific to calculating the network latency and packet count.
func NewLatencyObservers(m *Metrics) LatencyObservers {

	o := LatencyObservers{
		log:             m.log,
		requestsCounter: RequestsCounterMetric(),
		requestsLatency: RequestsLatencyMetric(),
		serviceName:     strings.ToLower(m.serviceName),
		labels:          m.labels,
	}

	m.Register(o.requestsCounter, o.requestsLatency)
	return o
}

func (o *LatencyObservers) ObserveRequestsCount(fields []string, method string) {

	for _, label := range o.labels {
		if len(fields) > label.offset && fields[label.offset] == label.label {
			o.log.Infof("Count %s: %s", label.label, method)
			o.requestsCounter.WithLabelValues(method, o.serviceName, label.label).Inc()
			return
		}
	}
}

func (o *LatencyObservers) ObserveRequestsLatency(elapsed float64, fields []string, method string) {

	for _, label := range o.labels {
		if len(fields) > label.offset && fields[label.offset] == label.label {
			o.log.Infof("Latency %v %s: %s", elapsed, label.label, method)
			o.requestsLatency.WithLabelValues(method, o.serviceName, label.label).Observe(elapsed)
			return
		}
	}
}
