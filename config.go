package ginshow

import (
	"time"
)

// DefaultBasePath is the default route prefix for ginshow endpoints.
// It is intentionally non-obvious. Override DashboardPath, MetricsPath,
// and PprofPrefix in production, and always combine with Auth in Production().
const DefaultBasePath = "/__gs/x7f3a2c9"

// Config controls profiling endpoints and request monitoring.
type Config struct {
	// EnablePprof exposes standard net/http/pprof handlers.
	EnablePprof bool

	// PprofPrefix is the route group prefix for pprof endpoints.
	// Default: {DefaultBasePath}/pprof
	PprofPrefix string

	// EnableMetrics exposes runtime metrics as JSON.
	EnableMetrics bool

	// MetricsPath is the JSON metrics endpoint path.
	// Default: {DefaultBasePath}/metrics
	MetricsPath string

	// EnableDashboard serves a single-file HTML monitoring UI.
	EnableDashboard bool

	// DashboardPath is the monitoring UI page path.
	// Default: DefaultBasePath
	DashboardPath string

	// DashboardTitle is the page title shown in the UI header.
	DashboardTitle string

	// EnableMiddleware collects per-request stats and optional slow-request logs.
	EnableMiddleware bool

	// SlowRequestThreshold logs requests slower than this duration.
	// Zero disables slow-request logging.
	SlowRequestThreshold time.Duration

	// Auth protects all debug endpoints when set.
	Auth *AuthConfig

	// BlockProfileRate enables block profiling when > 0.
	// See runtime.SetBlockProfileRate.
	BlockProfileRate int

	// MutexProfileFraction enables mutex profiling when > 0.
	// See runtime.SetMutexProfileFraction.
	MutexProfileFraction int
}

// AuthConfig protects debug routes with HTTP Basic Auth.
type AuthConfig struct {
	Username string
	Password string
}

// Default returns a safe local-development configuration.
func Default() Config {
	return Config{
		EnablePprof:          true,
		PprofPrefix:          DefaultBasePath + "/pprof",
		EnableMetrics:        true,
		MetricsPath:          DefaultBasePath + "/metrics",
		EnableDashboard:      true,
		DashboardPath:        DefaultBasePath,
		EnableMiddleware:     true,
		SlowRequestThreshold: 500 * time.Millisecond,
	}
}

// Production returns a configuration suitable for production:
// pprof and metrics enabled but protected by basic auth.
func Production(username, password string) Config {
	cfg := Default()
	cfg.Auth = &AuthConfig{
		Username: username,
		Password: password,
	}
	return cfg
}

func (c Config) withDefaults() Config {
	if c.PprofPrefix == "" {
		c.PprofPrefix = DefaultBasePath + "/pprof"
	}
	if c.MetricsPath == "" {
		c.MetricsPath = DefaultBasePath + "/metrics"
	}
	if c.DashboardPath == "" {
		c.DashboardPath = DefaultBasePath
	}
	return c
}
