package metrics

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/datatrails/go-datatrails-common/environment"
)

const (
	TenanciesIdentityPrefix = "tenant/"
)

// CosmosChargeMetric measures cosmos request charge per tenant using inbuilt Set method
func CosmosChargeMetric() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "archivist_cosmos_charge",
			Help: "Cosmos charge by tenant, method and resource.",
		},
		[]string{"tenant", "method", "resource"},
	)
}

// CosmosDurationMetric measures cosmos request duration(ms) per tenant using inbuilt Set method
func CosmosDurationMetric() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "archivist_cosmos_duration",
			Help: "Cosmos duration by tenant, method and resource.",
		},
		[]string{"tenant", "method", "resource"},
	)
}

// RequestsCounterMetric measures consumption per tenant
func RequestsCounterMetric() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "archivist_requests_total",
			Help: "Total number of requests by method, tenant, service and resource.",
		},
		[]string{"method", "tenant", "service", "resource"},
	)
}

// RequestsLatencyMetric measures an SLA "95% of all requests must be made in less than 100ms" and to
// plot average response latency and the apdex score.
// https://www.bookstack.cn/read/prometheus-en/1e87bb1c6ea1f003.md
// bucket limits are in seconds...
func RequestsLatencyMetric() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "archivist_requests_latency",
			Help:    "Histogram of time to reply to request.",
			Buckets: []float64{.005, .01, .02, .04, .08, .16, .32},
		},
		[]string{"method", "tenant", "service", "resource"},
	)
}

// create metric according to proof mechanism of ledger or tenant_storage
// EventsConfirmDurationMetric measures an SLA "95% of all confirmations must be made in less than 5minutes" and to
// plot average confirmation time and the apdex score.
// https://www.bookstack.cn/read/prometheus-en/1e87bb1c6ea1f003.md
// bucket limits are in seconds... and are different for ledger and tenant_storage
func NewEventsConfirmDurationMetric(name string, buckets []float64) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    fmt.Sprintf("archivist_%sevents_confirmation_duration", name),
			Help:    "Histogram of time to confirm an event.",
			Buckets: buckets,
		},
		[]string{"tenant", "operation"},
	)
}

// Metrics. Only those metrics specified
// are returned. The GoCollector and ProcessCollector metrics are omitted by
// using our own registry.
type Metrics struct {
	serviceName string
	port        string
	registry    *prometheus.Registry
	labels      []latencyObserveOffset
	log         Logger
}

type MetricsOption func(*Metrics)

func WithLabel(label string, offset int) MetricsOption {
	return func(m *Metrics) {
		m.labels = append(m.labels, latencyObserveOffset{label: label, offset: offset})
	}
}

func New(log Logger, serviceName string, opts ...MetricsOption) *Metrics {
	m := Metrics{
		log:         log,
		serviceName: strings.ToLower(serviceName),
		registry:    prometheus.NewRegistry(),
		labels:      []latencyObserveOffset{},
	}
	for _, opt := range opts {
		opt(&m)
	}
	return &m
}

func NewFromEnvironment(log Logger, serviceName string, opts ...MetricsOption) *Metrics {
	useMetrics := environment.GetTruthyOrFatal("USE_METRICS")
	var port string
	if useMetrics {
		port = environment.GetOrFatal("METRICS_PORT")
	}
	var m *Metrics
	if port != "" {
		m = New(
			log,
			serviceName,
			opts...,
		)
		m.port = port
	}
	return m
}

func (m *Metrics) String() string {
	return m.serviceName
}

func (m *Metrics) Register(cs ...prometheus.Collector) {
	m.registry.MustRegister(cs...)
}

func (m *Metrics) Port() string {
	if m != nil {
		return m.port
	}
	return ""
}

// The following code is only for restproxy endpoints and allows the propagation of
// the tenant Id from the underlying GRPC service.

// NewPromHandler - this handler is used on the endpoint that serves metrics endpoint
// which is provided on a different port to the service.
// The default InstrumentMetricHandler is suppressed.
func (m *Metrics) NewPromHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
