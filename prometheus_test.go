package ginshow_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
)

func TestPrometheusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = true
	ginshow.Mount(r, cfg)

	r.GET("/api/users/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})
	r.GET("/api/fail", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	for _, p := range []string{"/api/users/1", "/api/users/2", "/api/fail"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodGet, cfg.Prometheus.Path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, needle := range []string{
		"ginshow_http_requests_total",
		`route="/api/users/:id"`,
		`code="200"`,
		`code="500"`,
		"ginshow_http_request_duration_seconds",
		"ginshow_http_errors_5xx_total",
		"ginshow_http_request_duration_p95_seconds",
		"ginshow_http_request_duration_p99_seconds",
		"ginshow_goroutines",
		"go_goroutines",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("prometheus body missing %q\n%s", needle, body)
		}
	}
}

func TestPrometheusSkippedByMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.Prometheus.Path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	body, err := ginshow.MetricsJSON()
	if err != nil {
		t.Fatalf("MetricsJSON failed: %v", err)
	}
	if strings.Contains(string(body), `"total_requests":1`) || strings.Contains(string(body), `"total_requests": 1`) {
		t.Fatalf("prometheus scrape should not be counted as app request")
	}
}

func TestPrometheusAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Production("admin", "secret")
	cfg.EnableMiddleware = false
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.Prometheus.Path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with auth, got %d", rec.Code)
	}
}
