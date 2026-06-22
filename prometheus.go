package ginshow

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusConfig configures Prometheus exposition.
type PrometheusConfig struct {
	// Enable exposes Prometheus metrics endpoint.
	Enable bool

	// Path is the scrape endpoint. Default: {DefaultBasePath}/prometheus
	Path string

	// Namespace is the metric name prefix. Default: ginshow
	Namespace string

	// DisableGoCollector skips Go runtime metrics from prometheus.
	DisableGoCollector bool

	// DisableProcessCollector skips process metrics from prometheus.
	DisableProcessCollector bool
}

// DefaultPrometheus returns default Prometheus settings.
func DefaultPrometheus() PrometheusConfig {
	return PrometheusConfig{
		Enable:    true,
		Path:      DefaultBasePath + "/prometheus",
		Namespace: "ginshow",
	}
}

type promBundle struct {
	registry          *prometheus.Registry
	requestsTotal     *prometheus.CounterVec
	requestDuration   *prometheus.HistogramVec
	slowTotal         *prometheus.CounterVec
	errors5xxTotal    *prometheus.CounterVec
	panicsTotal       *prometheus.CounterVec
	inFlight          prometheus.Gauge
	requestP95Seconds prometheus.GaugeFunc
	requestP99Seconds prometheus.GaugeFunc
	goroutines        prometheus.GaugeFunc
	heapAlloc         prometheus.GaugeFunc
	uptime            prometheus.GaugeFunc
}

var (
	globalProm   *promBundle
	globalPromMu sync.RWMutex
)

func (p PrometheusConfig) withDefaults() PrometheusConfig {
	if p.Path == "" {
		p.Path = DefaultBasePath + "/prometheus"
	}
	if p.Namespace == "" {
		p.Namespace = "ginshow"
	}
	return p
}

func (p PrometheusConfig) enabled() bool {
	return p.Enable
}

func setupPrometheus(cfg PrometheusConfig) {
	cfg = cfg.withDefaults()

	globalPromMu.Lock()
	defer globalPromMu.Unlock()

	if globalProm != nil {
		return
	}

	reg := prometheus.NewRegistry()
	ns := cfg.Namespace

	bundle := &promBundle{
		registry: reg,
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "http_requests_total",
			Help:      "Total HTTP requests grouped by method, route template and status code.",
		}, []string{"method", "route", "code"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency grouped by method and route template.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"method", "route"}),
		slowTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "http_slow_requests_total",
			Help:      "Total slow HTTP requests grouped by method and route template.",
		}, []string{"method", "route"}),
		errors5xxTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "http_errors_5xx_total",
			Help:      "Total HTTP 5xx responses grouped by method and route template.",
		}, []string{"method", "route"}),
		panicsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "http_panics_total",
			Help:      "Total recovered panics grouped by method and route template.",
		}, []string{"method", "route"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "http_requests_in_flight",
			Help:      "Current in-flight HTTP requests.",
		}),
		goroutines: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "goroutines",
			Help:      "Current number of goroutines.",
		}, func() float64 {
			return float64(collectRuntimeMetrics().NumGoroutine)
		}),
		requestP95Seconds: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "http_request_duration_p95_seconds",
			Help:      "Approximate P95 HTTP request latency in seconds.",
		}, func() float64 {
			return globalStats.snapshot().P95LatencyMs / 1000
		}),
		requestP99Seconds: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "http_request_duration_p99_seconds",
			Help:      "Approximate P99 HTTP request latency in seconds.",
		}, func() float64 {
			return globalStats.snapshot().P99LatencyMs / 1000
		}),
		heapAlloc: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "memory_heap_alloc_bytes",
			Help:      "Current heap allocation in bytes.",
		}, func() float64 {
			return float64(collectRuntimeMetrics().Memory.HeapAlloc)
		}),
		uptime: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "process_uptime_seconds",
			Help:      "Process uptime in seconds.",
		}, func() float64 {
			return time.Since(globalStats.startedAt).Seconds()
		}),
	}

	collectorsToRegister := []prometheus.Collector{
		bundle.requestsTotal,
		bundle.requestDuration,
		bundle.slowTotal,
		bundle.errors5xxTotal,
		bundle.panicsTotal,
		bundle.inFlight,
		bundle.requestP95Seconds,
		bundle.requestP99Seconds,
		bundle.goroutines,
		bundle.heapAlloc,
		bundle.uptime,
	}
	if !cfg.DisableGoCollector {
		collectorsToRegister = append(collectorsToRegister, collectors.NewGoCollector())
	}
	if !cfg.DisableProcessCollector {
		collectorsToRegister = append(collectorsToRegister, collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}
	reg.MustRegister(collectorsToRegister...)
	globalProm = bundle
}

func prometheusHandler() gin.HandlerFunc {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		globalPromMu.RLock()
		bundle := globalProm
		globalPromMu.RUnlock()
		if bundle == nil {
			http.Error(w, "prometheus not enabled", http.StatusNotFound)
			return
		}
		promhttp.HandlerFor(bundle.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})

	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

func promRouteLabels(method, fullPath string) (string, string) {
	route := fullPath
	if route == "" {
		route = "__unmatched__"
	}
	return method, route
}

func promBeginRequest() {
	globalPromMu.RLock()
	bundle := globalProm
	globalPromMu.RUnlock()
	if bundle == nil {
		return
	}
	bundle.inFlight.Inc()
}

func promEndRequest() {
	globalPromMu.RLock()
	bundle := globalProm
	globalPromMu.RUnlock()
	if bundle == nil {
		return
	}
	bundle.inFlight.Dec()
}

func promRecordRequest(method, fullPath string, status int, duration time.Duration, slow, panicked bool) {
	globalPromMu.RLock()
	bundle := globalProm
	globalPromMu.RUnlock()
	if bundle == nil {
		return
	}

	m, route := promRouteLabels(method, fullPath)
	code := strconv.Itoa(status)

	bundle.requestsTotal.WithLabelValues(m, route, code).Inc()
	bundle.requestDuration.WithLabelValues(m, route).Observe(duration.Seconds())
	if slow {
		bundle.slowTotal.WithLabelValues(m, route).Inc()
	}
	if status >= 500 {
		bundle.errors5xxTotal.WithLabelValues(m, route).Inc()
	}
	if panicked {
		bundle.panicsTotal.WithLabelValues(m, route).Inc()
	}
}
