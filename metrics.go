package ginshow

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// RuntimeMetrics is a JSON snapshot of Go runtime state.
type RuntimeMetrics struct {
	Timestamp   time.Time     `json:"timestamp"`
	GoVersion   string        `json:"go_version"`
	NumCPU      int           `json:"num_cpu"`
	NumGoroutine int          `json:"num_goroutine"`
	NumCgoCall  int64         `json:"num_cgo_call"`
	Memory      MemoryMetrics `json:"memory"`
	GC          GCMetrics     `json:"gc"`
	Requests    RequestStats  `json:"requests"`
}

type MemoryMetrics struct {
	Alloc        uint64 `json:"alloc_bytes"`
	TotalAlloc   uint64 `json:"total_alloc_bytes"`
	Sys          uint64 `json:"sys_bytes"`
	HeapAlloc    uint64 `json:"heap_alloc_bytes"`
	HeapSys      uint64 `json:"heap_sys_bytes"`
	HeapInuse    uint64 `json:"heap_inuse_bytes"`
	HeapIdle     uint64 `json:"heap_idle_bytes"`
	HeapReleased uint64 `json:"heap_released_bytes"`
	StackInuse   uint64 `json:"stack_inuse_bytes"`
	StackSys     uint64 `json:"stack_sys_bytes"`
}

type GCMetrics struct {
	NumGC        uint32  `json:"num_gc"`
	LastGC       string  `json:"last_gc"`
	PauseTotalNs uint64  `json:"pause_total_ns"`
	PauseNs      uint64  `json:"last_pause_ns"`
	GCCPUFraction float64 `json:"gc_cpu_fraction"`
}

func collectRuntimeMetrics() RuntimeMetrics {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	lastGC := ""
	if ms.LastGC > 0 {
		lastGC = time.Unix(0, int64(ms.LastGC)).Format(time.RFC3339Nano)
	}

	return RuntimeMetrics{
		Timestamp:    time.Now().UTC(),
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCgoCall:   runtime.NumCgoCall(),
		Memory: MemoryMetrics{
			Alloc:        ms.Alloc,
			TotalAlloc:   ms.TotalAlloc,
			Sys:          ms.Sys,
			HeapAlloc:    ms.HeapAlloc,
			HeapSys:      ms.HeapSys,
			HeapInuse:    ms.HeapInuse,
			HeapIdle:     ms.HeapIdle,
			HeapReleased: ms.HeapReleased,
			StackInuse:   ms.StackInuse,
			StackSys:     ms.StackSys,
		},
		GC: GCMetrics{
			NumGC:         ms.NumGC,
			LastGC:        lastGC,
			PauseTotalNs:  ms.PauseTotalNs,
			PauseNs:       ms.PauseNs[(ms.NumGC+255)%256],
			GCCPUFraction: ms.GCCPUFraction,
		},
		Requests: globalStats.snapshot(),
	}
}

func metricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, collectRuntimeMetrics())
	}
}

// MetricsJSON returns the current runtime metrics snapshot.
func MetricsJSON() ([]byte, error) {
	return json.Marshal(collectRuntimeMetrics())
}
