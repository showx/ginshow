package ginshow_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
)

func TestMountRegistersDebugEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = false
	ginshow.Mount(r, cfg)

	tests := []struct {
		path string
	}{
		{cfg.DashboardPath},
		{cfg.PprofPrefix + "/"},
		{cfg.PprofPrefix + "/heap"},
		{cfg.MetricsPath},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
		})
	}
}

func TestMetricsJSONContainsRuntimeFields(t *testing.T) {
	body, err := ginshow.MetricsJSON()
	if err != nil {
		t.Fatalf("MetricsJSON failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	for _, key := range []string{"go_version", "num_goroutine", "memory", "gc", "requests"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing key %q", key)
		}
	}
}

func TestMiddlewareSkipsDebugRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.SlowRequestThreshold = time.Millisecond
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.MetricsPath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	statsBody, err := ginshow.MetricsJSON()
	if err != nil {
		t.Fatalf("MetricsJSON failed: %v", err)
	}

	var payload struct {
		Requests ginshow.RequestStats `json:"requests"`
	}
	if err := json.Unmarshal(statsBody, &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if payload.Requests.TotalRequests != 0 {
		t.Fatalf("expected debug route to be excluded, total=%d", payload.Requests.TotalRequests)
	}
}

func TestDashboardContainsEmbeddedUI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = false
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.DashboardPath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", ct)
	}
	body := rec.Body.String()
	for _, needle := range []string{"ginshow", "pprof 快捷入口", "const CFG =", "metricsPath"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("dashboard missing %q", needle)
		}
	}
}

func TestProductionAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Production("admin", "secret")
	cfg.EnableMiddleware = false
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.MetricsPath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}

	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with auth, got %d", rec.Code)
	}
}
