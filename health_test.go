package ginshow_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
)

func TestHealthzReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = false
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.Health.HealthzPath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("unexpected status: %v", payload["status"])
	}
}

func TestReadyzWithoutChecks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = false
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.Health.ReadyzPath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.Status != "ready" {
		t.Fatalf("unexpected status: %s", payload.Status)
	}
}

func TestReadyzWithFailingCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = false
	cfg.Health.Checks = []ginshow.NamedCheck{
		ginshow.SimpleCheck("db", func(context.Context) error {
			return errors.New("connection refused")
		}),
	}
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.Health.ReadyzPath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var payload struct {
		Status string `json:"status"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.Status != "not_ready" {
		t.Fatalf("unexpected status: %s", payload.Status)
	}
	if len(payload.Checks) != 1 || payload.Checks[0].Status != "fail" {
		t.Fatalf("unexpected checks: %+v", payload.Checks)
	}
}

func TestHealthEndpointsSkippedByMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.SlowRequestThreshold = 1
	ginshow.Mount(r, cfg)

	for _, path := range []string{cfg.Health.HealthzPath, cfg.Health.ReadyzPath} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("path %s expected 200, got %d", path, rec.Code)
		}
	}

	body, err := ginshow.MetricsJSON()
	if err != nil {
		t.Fatalf("MetricsJSON failed: %v", err)
	}

	var payload struct {
		Requests ginshow.RequestStats `json:"requests"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.Requests.TotalRequests != 0 {
		t.Fatalf("health routes should be excluded, total=%d", payload.Requests.TotalRequests)
	}
}
