package ginshow

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

//go:embed dashboard.html
var dashboardFS embed.FS

var (
	dashboardTmpl     *template.Template
	dashboardTmplOnce sync.Once
	dashboardTmplErr  error
)

type dashboardPage struct {
	Title       string
	ConfigJSON  template.JS
	RequireAuth bool
	LoginPath   string
}

func loadDashboardTemplate() (*template.Template, error) {
	dashboardTmplOnce.Do(func() {
		dashboardTmpl, dashboardTmplErr = template.ParseFS(dashboardFS, "dashboard.html")
	})
	return dashboardTmpl, dashboardTmplErr
}

func dashboardHandler(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tmpl, err := loadDashboardTemplate()
		if err != nil {
			c.String(http.StatusInternalServerError, "dashboard template error: %v", err)
			return
		}

		configJSON, err := json.Marshal(map[string]any{
			"metricsPath": cfg.MetricsPath,
			"pprofPrefix": cfg.PprofPrefix,
			"requireAuth": cfg.Auth != nil && cfg.Auth.Username != "",
			"loginPath":   dashboardLoginPath(cfg),
		})
		if err != nil {
			c.String(http.StatusInternalServerError, "dashboard config error: %v", err)
			return
		}

		title := cfg.DashboardTitle
		if title == "" {
			title = "ginshow 监控面板"
		}

		var buf bytes.Buffer
		requireAuth := cfg.Auth != nil && cfg.Auth.Username != ""
		if err := tmpl.Execute(&buf, dashboardPage{
			Title:       title,
			ConfigJSON:  template.JS(configJSON),
			RequireAuth: requireAuth,
			LoginPath:   dashboardLoginPath(cfg),
		}); err != nil {
			c.String(http.StatusInternalServerError, "dashboard render error: %v", err)
			return
		}

		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
	}
}

func isInternalDebugPath(path string, cfg Config) bool {
	if strings.HasPrefix(path, cfg.PprofPrefix) {
		return true
	}
	if path == cfg.MetricsPath {
		return true
	}
	prom := cfg.Prometheus.withDefaults()
	if prom.Enable && path == prom.Path {
		return true
	}
	if cfg.EnableDashboard && path == cfg.DashboardPath {
		return true
	}
	if cfg.EnableDashboard && path == dashboardLoginPath(cfg) {
		return true
	}
	health := cfg.Health.withDefaults()
	if health.EnableHealthz && path == health.HealthzPath {
		return true
	}
	if health.EnableReadyz && path == health.ReadyzPath {
		return true
	}
	return false
}
