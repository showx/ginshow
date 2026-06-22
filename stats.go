package ginshow

import (
	"sync/atomic"
	"time"
)

var globalStats statsCollector

type statsCollector struct {
	startedAt       time.Time
	totalRequests   atomic.Uint64
	inFlight        atomic.Int64
	slowRequests    atomic.Uint64
	totalDurationNs atomic.Uint64
}

func init() {
	globalStats.startedAt = time.Now()
}

func (s *statsCollector) recordRequest(duration time.Duration, slow bool) {
	s.totalRequests.Add(1)
	s.totalDurationNs.Add(uint64(duration.Nanoseconds()))
	if slow {
		s.slowRequests.Add(1)
	}
}

func (s *statsCollector) beginRequest() {
	s.inFlight.Add(1)
}

func (s *statsCollector) endRequest() {
	s.inFlight.Add(-1)
}

func (s *statsCollector) snapshot() RequestStats {
	total := s.totalRequests.Load()
	var avgMs float64
	if total > 0 {
		avgMs = float64(s.totalDurationNs.Load()) / float64(total) / float64(time.Millisecond)
	}

	return RequestStats{
		Uptime:        time.Since(s.startedAt).String(),
		TotalRequests: total,
		InFlight:      s.inFlight.Load(),
		SlowRequests:  s.slowRequests.Load(),
		AvgLatencyMs:  avgMs,
	}
}

// RequestStats summarizes HTTP request monitoring data.
type RequestStats struct {
	Uptime        string  `json:"uptime"`
	TotalRequests uint64  `json:"total_requests"`
	InFlight      int64   `json:"in_flight"`
	SlowRequests  uint64  `json:"slow_requests"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
}
