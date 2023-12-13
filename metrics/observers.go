package metrics

import (
	"net/url"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

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

type LatencyOption func(*LatencyObservers)

func WithLabel(label string, offset int) LatencyOption {
	return func(l *LatencyObservers) {
		l.labels = append(l.labels, latencyObserveOffset{label: label, offset: offset})
	}
}

// NewLatencyObservers is specific to calculating the network latency and packet count.
func NewLatencyObservers(m *Metrics, opts ...LatencyOption) LatencyObservers {

	o := LatencyObservers{
		log:             m.log,
		requestsCounter: RequestsCounterMetric(),
		requestsLatency: RequestsLatencyMetric(),
		serviceName:     strings.ToLower(m.serviceName),
		labels:          m.labels,
	}
	for _, opt := range opts {
		opt(&o)
	}

	m.Register(o.requestsCounter, o.requestsLatency)
	return o
}

// statusCode is not of interest
func (o *LatencyObservers) ObserveRequestsCount(_ *url.URL, fields []string, _ string, method string, tenant string) {

	for _, label := range o.labels {
		if len(fields) > label.offset && fields[label.offset] == label.label {
			o.log.Infof("Count %s: %s, %s", label.label, method, tenant)
			o.requestsCounter.WithLabelValues(method, tenant, o.serviceName, label.label).Inc()
			return
		}
	}
}

func (o *LatencyObservers) ObserveRequestsLatency(elapsed float64, fields []string, method string, tenant string) {

	for _, label := range o.labels {
		if len(fields) > label.offset && fields[label.offset] == label.label {
			o.log.Infof("Latency %v %s: %s, %s", elapsed, label.label, method, tenant)
			o.requestsLatency.WithLabelValues(method, tenant, o.serviceName, label.label).Observe(elapsed)
			return
		}
	}
}
