package ginshow

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ReadinessCheck verifies a dependency during readyz probing.
type ReadinessCheck func(ctx context.Context) error

// NamedCheck is a named readiness dependency.
type NamedCheck struct {
	Name   string
	Check  ReadinessCheck
	Detail func(ctx context.Context) any
}

// HealthConfig configures /healthz and /readyz endpoints.
type HealthConfig struct {
	// EnableHealthz exposes the liveness endpoint.
	EnableHealthz bool

	// EnableReadyz exposes the readiness endpoint.
	EnableReadyz bool

	// HealthzPath is the liveness probe path. Default: /healthz
	HealthzPath string

	// ReadyzPath is the readiness probe path. Default: /readyz
	ReadyzPath string

	// Checks are evaluated by readyz. All must pass to return ready.
	Checks []NamedCheck

	// CheckTimeout limits each readiness check. Default: 3s
	CheckTimeout time.Duration
}

// CheckReport is the result of a single readiness check.
type CheckReport struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	LatencyMs float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
	Detail    any     `json:"detail,omitempty"`
}

type healthzResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type readyzResponse struct {
	Status    string        `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Checks    []CheckReport `json:"checks"`
}

// DefaultHealth returns default health probe settings.
func DefaultHealth() HealthConfig {
	return HealthConfig{
		EnableHealthz: true,
		EnableReadyz:  true,
		HealthzPath:   "/healthz",
		ReadyzPath:    "/readyz",
		CheckTimeout:  3 * time.Second,
	}
}

// SimpleCheck wraps a readiness function as a named check.
func SimpleCheck(name string, check ReadinessCheck) NamedCheck {
	return NamedCheck{Name: name, Check: check}
}

func (h HealthConfig) withDefaults() HealthConfig {
	if h.HealthzPath == "" {
		h.HealthzPath = "/healthz"
	}
	if h.ReadyzPath == "" {
		h.ReadyzPath = "/readyz"
	}
	if h.CheckTimeout == 0 {
		h.CheckTimeout = 3 * time.Second
	}
	return h
}

func (h HealthConfig) enabled() bool {
	return h.EnableHealthz || h.EnableReadyz
}

func registerHealthRoutes(r gin.IRouter, cfg HealthConfig) {
	cfg = cfg.withDefaults()

	if cfg.EnableHealthz {
		r.GET(cfg.HealthzPath, healthzHandler())
	}
	if cfg.EnableReadyz {
		r.GET(cfg.ReadyzPath, readyzHandler(cfg))
	}
}

func healthzHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, healthzResponse{
			Status:    "ok",
			Timestamp: time.Now().UTC(),
		})
	}
}

func readyzHandler(cfg HealthConfig) gin.HandlerFunc {
	cfg = cfg.withDefaults()

	return func(c *gin.Context) {
		reports := runReadinessChecks(c.Request.Context(), cfg)
		status := "ready"
		code := http.StatusOK
		for _, r := range reports {
			if r.Status != "ok" {
				status = "not_ready"
				code = http.StatusServiceUnavailable
				break
			}
		}

		c.Header("Cache-Control", "no-store")
		c.JSON(code, readyzResponse{
			Status:    status,
			Timestamp: time.Now().UTC(),
			Checks:    reports,
		})
	}
}

func runReadinessChecks(parent context.Context, cfg HealthConfig) []CheckReport {
	if len(cfg.Checks) == 0 {
		return []CheckReport{{
			Name:   "self",
			Status: "ok",
		}}
	}

	reports := make([]CheckReport, 0, len(cfg.Checks))
	for _, item := range cfg.Checks {
		reports = append(reports, runNamedCheck(parent, cfg.CheckTimeout, item))
	}
	return reports
}

func runNamedCheck(parent context.Context, timeout time.Duration, item NamedCheck) CheckReport {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	start := time.Now()
	report := CheckReport{
		Name: item.Name,
	}

	err := item.Check(ctx)
	report.LatencyMs = float64(time.Since(start).Microseconds()) / 1000

	if err != nil {
		report.Status = "fail"
		report.Error = err.Error()
		return report
	}

	report.Status = "ok"
	if item.Detail != nil {
		report.Detail = item.Detail(ctx)
	}
	return report
}
