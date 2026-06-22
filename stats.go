package ginshow

import (
	"math"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const defaultLatencySampleCap = 4096

var globalStats statsCollector

type statsCollector struct {
	startedAt       time.Time
	totalRequests   atomic.Uint64
	inFlight        atomic.Int64
	slowRequests    atomic.Uint64
	totalDurationNs atomic.Uint64
	errors5xx       atomic.Uint64
	errors4xx       atomic.Uint64
	panics          atomic.Uint64
	statusCodes     [600]atomic.Uint64
	latencies       latencyRing
	routes          sync.Map // route key -> *routeStat
}

type routeStat struct {
	requests        atomic.Uint64
	slowRequests    atomic.Uint64
	totalDurationNs atomic.Uint64
	errors5xx       atomic.Uint64
	errors4xx       atomic.Uint64
	panics          atomic.Uint64
	statusCodes     [600]atomic.Uint64
	latencies       latencyRing
}

type latencyRing struct {
	mu   sync.Mutex
	data []float64
	idx  int
}

func init() {
	globalStats.startedAt = time.Now()
}

func routeKey(method, fullPath string) string {
	if fullPath == "" {
		return method + " __unmatched__"
	}
	return method + " " + fullPath
}

func (l *latencyRing) add(ms float64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.data) < defaultLatencySampleCap {
		l.data = append(l.data, ms)
		return
	}

	l.data[l.idx] = ms
	l.idx = (l.idx + 1) % defaultLatencySampleCap
}

func (l *latencyRing) percentiles() (p95, p99 float64) {
	l.mu.Lock()
	samples := append([]float64(nil), l.data...)
	l.mu.Unlock()

	if len(samples) == 0 {
		return 0, 0
	}

	sort.Float64s(samples)
	return percentile(samples, 95), percentile(samples, 99)
}

func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}

	rank := int(math.Ceil(p/100*float64(n))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= n {
		rank = n - 1
	}
	return sorted[rank]
}

func durationToMs(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / float64(time.Millisecond)
}

func (s *statsCollector) beginRequest() {
	s.inFlight.Add(1)
}

func (s *statsCollector) endRequest() {
	s.inFlight.Add(-1)
}

func (s *statsCollector) recordRequest(method, fullPath string, status int, duration time.Duration, slow, panicked bool) {
	s.totalRequests.Add(1)
	s.totalDurationNs.Add(uint64(duration.Nanoseconds()))
	if slow {
		s.slowRequests.Add(1)
	}
	if panicked {
		s.panics.Add(1)
		status = httpStatusOrDefault(status, 500)
	}

	recordStatus(&s.statusCodes, status)
	if status >= 500 {
		s.errors5xx.Add(1)
	} else if status >= 400 {
		s.errors4xx.Add(1)
	}

	s.latencies.add(durationToMs(duration))

	key := routeKey(method, fullPath)
	val, _ := s.routes.LoadOrStore(key, &routeStat{})
	rs := val.(*routeStat)
	rs.record(status, duration, slow, panicked)
}

func (rs *routeStat) record(status int, duration time.Duration, slow, panicked bool) {
	rs.requests.Add(1)
	rs.totalDurationNs.Add(uint64(duration.Nanoseconds()))
	if slow {
		rs.slowRequests.Add(1)
	}
	if panicked {
		rs.panics.Add(1)
		status = httpStatusOrDefault(status, 500)
	}

	recordStatus(&rs.statusCodes, status)
	if status >= 500 {
		rs.errors5xx.Add(1)
	} else if status >= 400 {
		rs.errors4xx.Add(1)
	}

	rs.latencies.add(durationToMs(duration))
}

func httpStatusOrDefault(status, fallback int) int {
	if status == 0 {
		return fallback
	}
	return status
}

func recordStatus(codes *[600]atomic.Uint64, status int) {
	if status >= 0 && status < len(codes) {
		codes[status].Add(1)
	}
}

func (s *statsCollector) snapshot() RequestStats {
	total := s.totalRequests.Load()
	var avgMs float64
	if total > 0 {
		avgMs = float64(s.totalDurationNs.Load()) / float64(total) / float64(time.Millisecond)
	}
	p95, p99 := s.latencies.percentiles()

	return RequestStats{
		Uptime:          time.Since(s.startedAt).String(),
		TotalRequests:   total,
		InFlight:        s.inFlight.Load(),
		SlowRequests:    s.slowRequests.Load(),
		AvgLatencyMs:    avgMs,
		P95LatencyMs:    p95,
		P99LatencyMs:    p99,
		Errors5xx:       s.errors5xx.Load(),
		Panics:          s.panics.Load(),
		ErrorRate:       rate(s.errors5xx.Load(), total),
		PanicRate:       rate(s.panics.Load(), total),
		ClientErrorRate: rate(s.errors4xx.Load(), total),
		StatusCodes:     snapshotStatusCodes(&s.statusCodes),
		Routes:          s.snapshotRoutes(),
	}
}

func (s *statsCollector) snapshotRoutes() []RouteStats {
	items := make([]RouteStats, 0)
	s.routes.Range(func(key, value any) bool {
		route := key.(string)
		rs := value.(*routeStat)
		total := rs.requests.Load()
		var avgMs float64
		if total > 0 {
			avgMs = float64(rs.totalDurationNs.Load()) / float64(total) / float64(time.Millisecond)
		}
		p95, p99 := rs.latencies.percentiles()

		items = append(items, RouteStats{
			Route:           route,
			Requests:        total,
			SlowRequests:    rs.slowRequests.Load(),
			AvgLatencyMs:    avgMs,
			P95LatencyMs:    p95,
			P99LatencyMs:    p99,
			Errors5xx:       rs.errors5xx.Load(),
			Panics:          rs.panics.Load(),
			ErrorRate:       rate(rs.errors5xx.Load(), total),
			PanicRate:       rate(rs.panics.Load(), total),
			ClientErrorRate: rate(rs.errors4xx.Load(), total),
			StatusCodes:     snapshotStatusCodes(&rs.statusCodes),
		})
		return true
	})

	sort.Slice(items, func(i, j int) bool {
		if items[i].Requests == items[j].Requests {
			return items[i].Route < items[j].Route
		}
		return items[i].Requests > items[j].Requests
	})
	return items
}

func snapshotStatusCodes(codes *[600]atomic.Uint64) map[string]uint64 {
	out := make(map[string]uint64)
	for code, counter := range codes {
		count := counter.Load()
		if count == 0 {
			continue
		}
		out[strconv.Itoa(code)] = count
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func rate(part, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}

// RequestStats summarizes HTTP request monitoring data.
type RequestStats struct {
	Uptime          string            `json:"uptime"`
	TotalRequests   uint64            `json:"total_requests"`
	InFlight        int64             `json:"in_flight"`
	SlowRequests    uint64            `json:"slow_requests"`
	AvgLatencyMs    float64           `json:"avg_latency_ms"`
	P95LatencyMs    float64           `json:"p95_latency_ms"`
	P99LatencyMs    float64           `json:"p99_latency_ms"`
	Errors5xx       uint64            `json:"errors_5xx"`
	Panics          uint64            `json:"panics"`
	ErrorRate       float64           `json:"error_rate"`
	PanicRate       float64           `json:"panic_rate"`
	ClientErrorRate float64           `json:"client_error_rate"`
	StatusCodes     map[string]uint64 `json:"status_codes,omitempty"`
	Routes          []RouteStats      `json:"routes,omitempty"`
}

// RouteStats summarizes metrics grouped by Gin route template (FullPath).
type RouteStats struct {
	Route           string            `json:"route"`
	Requests        uint64            `json:"requests"`
	SlowRequests    uint64            `json:"slow_requests"`
	AvgLatencyMs    float64           `json:"avg_latency_ms"`
	P95LatencyMs    float64           `json:"p95_latency_ms"`
	P99LatencyMs    float64           `json:"p99_latency_ms"`
	Errors5xx       uint64            `json:"errors_5xx"`
	Panics          uint64            `json:"panics"`
	ErrorRate       float64           `json:"error_rate"`
	PanicRate       float64           `json:"panic_rate"`
	ClientErrorRate float64           `json:"client_error_rate"`
	StatusCodes     map[string]uint64 `json:"status_codes,omitempty"`
}
