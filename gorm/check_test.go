package gorm_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
	gshowgorm "github.com/showx/ginshow/gorm"
	"github.com/glebarez/sqlite"
	gormdb "gorm.io/gorm"
)

func TestGormCheckReady(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gormdb.Open(sqlite.Open(":memory:"), &gormdb.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = false
	cfg.Health.Checks = []ginshow.NamedCheck{
		gshowgorm.Check("primary", db),
	}
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.Health.ReadyzPath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Status string `json:"status"`
		Checks []struct {
			Name   string         `json:"name"`
			Status string         `json:"status"`
			Detail map[string]any `json:"detail"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.Status != "ready" {
		t.Fatalf("unexpected status: %s", payload.Status)
	}
	if len(payload.Checks) != 1 || payload.Checks[0].Name != "gorm:primary" {
		t.Fatalf("unexpected checks: %+v", payload.Checks)
	}
	if payload.Checks[0].Detail == nil {
		t.Fatalf("expected pool detail")
	}
}

func TestGormCheckFailsWhenNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = false
	cfg.Health.Checks = []ginshow.NamedCheck{
		gshowgorm.Check("primary", nil),
	}
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, cfg.Health.ReadyzPath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
