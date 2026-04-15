package observability

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics groups reusable process and HTTP instrumentation for services.
type Metrics struct {
	Service             string
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPInflight        *prometheus.GaugeVec
	DependencyUp        *prometheus.GaugeVec
	OutboxPublishes     *prometheus.CounterVec
}

// NewMetrics registers the default shared collectors for a service.
func NewMetrics(service string, registerer prometheus.Registerer) *Metrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	namespace := "wiki_editor"
	labels := prometheus.Labels{"service": service}

	m := &Metrics{
		Service: service,
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "http",
			Name:        "requests_total",
			Help:        "Total number of HTTP requests handled by the service.",
			ConstLabels: labels,
		}, []string{"method", "route", "status"}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "http",
			Name:        "request_duration_seconds",
			Help:        "Duration of handled HTTP requests.",
			ConstLabels: labels,
			Buckets:     prometheus.DefBuckets,
		}, []string{"method", "route", "status"}),
		HTTPInflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "http",
			Name:        "inflight_requests",
			Help:        "Current number of inflight HTTP requests.",
			ConstLabels: labels,
		}, []string{"route"}),
		DependencyUp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "runtime",
			Name:        "dependency_up",
			Help:        "Dependency health status where 1 is healthy and 0 is unhealthy.",
			ConstLabels: labels,
		}, []string{"dependency"}),
		OutboxPublishes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "outbox",
			Name:        "publish_total",
			Help:        "Number of outbox publish attempts grouped by final result.",
			ConstLabels: labels,
		}, []string{"result"}),
	}

	registerer.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPInflight,
		m.DependencyUp,
		m.OutboxPublishes,
	)

	return m
}

// InstrumentHTTP wraps a handler with standard request metrics.
func (m *Metrics) InstrumentHTTP(route string, next http.Handler) http.Handler {
	if m == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		m.HTTPInflight.WithLabelValues(route).Inc()
		startedAt := time.Now()

		defer func() {
			m.HTTPInflight.WithLabelValues(route).Dec()
			status := strconv.Itoa(ww.status)
			m.HTTPRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
			m.HTTPRequestDuration.WithLabelValues(r.Method, route, status).Observe(time.Since(startedAt).Seconds())
		}()

		next.ServeHTTP(ww, r)
	})
}

// SetDependency marks an infrastructure dependency as healthy or unhealthy.
func (m *Metrics) SetDependency(name string, healthy bool) {
	if m == nil {
		return
	}

	value := 0.0
	if healthy {
		value = 1
	}

	m.DependencyUp.WithLabelValues(name).Set(value)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}
