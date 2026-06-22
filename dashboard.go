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

		configJSON, err := json.Marshal(map[string]string{
			"metricsPath": cfg.MetricsPath,
			"pprofPrefix": cfg.PprofPrefix,
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
		if err := tmpl.Execute(&buf, dashboardPage{
			Title:      title,
			ConfigJSON: template.JS(configJSON),
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
	if cfg.EnableDashboard && path == cfg.DashboardPath {
		return true
	}
	return false
}
