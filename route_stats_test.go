package ginshow_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
)

func TestRouteStatsByTemplateNotURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	beforeBody, err := ginshow.MetricsJSON()
	if err != nil {
		t.Fatalf("MetricsJSON failed: %v", err)
	}
	var before struct {
		Requests ginshow.RequestStats `json:"requests"`
	}
	if err := json.Unmarshal(beforeBody, &before); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = true
	ginshow.Mount(r, cfg)

	r.GET("/api/users/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})
	r.GET("/api/error", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})
	r.GET("/api/missing", func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})

	paths := []string{"/api/users/1", "/api/users/2", "/api/users/999"}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/error", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	req = httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

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

	delta := payload.Requests.TotalRequests - before.Requests.TotalRequests
	if delta != 5 {
		t.Fatalf("expected 5 new requests, got %d (total=%d before=%d)",
			delta, payload.Requests.TotalRequests, before.Requests.TotalRequests)
	}

	routeMap := map[string]ginshow.RouteStats{}
	for _, item := range payload.Requests.Routes {
		routeMap[item.Route] = item
	}

	userRoute, ok := routeMap["GET /api/users/:id"]
	if !ok {
		t.Fatalf("missing route stats for template, routes=%v", payload.Requests.Routes)
	}
	if userRoute.Requests < 3 {
		t.Fatalf("expected at least 3 requests on user route, got %d", userRoute.Requests)
	}
	if userRoute.StatusCodes["200"] < 3 {
		t.Fatalf("expected at least 3x 200 on user route, got %v", userRoute.StatusCodes)
	}
	if _, ok := routeMap["GET /api/users/1"]; ok {
		t.Fatalf("should not stats by real url path")
	}

	errorRoute, ok := routeMap["GET /api/error"]
	if !ok || errorRoute.StatusCodes["500"] < 1 {
		t.Fatalf("expected 500 on error route, route=%+v", errorRoute)
	}

	missingRoute, ok := routeMap["GET /api/missing"]
	if !ok || missingRoute.StatusCodes["404"] < 1 {
		t.Fatalf("expected 404 on missing route, route=%+v", missingRoute)
	}
}

func TestUnmatchedRouteBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	ginshow.Mount(r, cfg)

	req := httptest.NewRequest(http.MethodGet, "/no/such/route", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

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

	found := false
	for _, item := range payload.Requests.Routes {
		if item.Route == "GET __unmatched__" {
			found = true
			if item.Requests < 1 {
				t.Fatalf("expected unmatched requests, got %d", item.Requests)
			}
		}
	}
	if !found {
		t.Fatalf("expected unmatched route bucket, routes=%v", payload.Requests.Routes)
	}
}

func TestPanicAnd5xxStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	beforeBody, err := ginshow.MetricsJSON()
	if err != nil {
		t.Fatalf("MetricsJSON failed: %v", err)
	}
	var before struct {
		Requests ginshow.RequestStats `json:"requests"`
	}
	if err := json.Unmarshal(beforeBody, &before); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	r := gin.Default()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = true
	ginshow.Mount(r, cfg)

	r.GET("/api/panic", func(c *gin.Context) {
		panic("boom")
	})
	r.GET("/api/fail", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	for _, p := range []string{"/api/panic", "/api/fail"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for %s, got %d", p, rec.Code)
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

	delta5xx := payload.Requests.Errors5xx - before.Requests.Errors5xx
	if delta5xx < 2 {
		t.Fatalf("expected at least 2 new 5xx, got %d", delta5xx)
	}

	deltaPanics := payload.Requests.Panics - before.Requests.Panics
	if deltaPanics < 1 {
		t.Fatalf("expected at least 1 panic, got %d", deltaPanics)
	}

	routeMap := map[string]ginshow.RouteStats{}
	for _, item := range payload.Requests.Routes {
		routeMap[item.Route] = item
	}

	panicRoute, ok := routeMap["GET /api/panic"]
	if !ok || panicRoute.Panics < 1 || panicRoute.Errors5xx < 1 {
		t.Fatalf("expected panic route stats, got %+v", panicRoute)
	}

	failRoute, ok := routeMap["GET /api/fail"]
	if !ok || failRoute.Errors5xx < 1 || failRoute.Panics != 0 {
		t.Fatalf("expected 5xx without panic, got %+v", failRoute)
	}
}

func TestLatencyPercentilesInMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	cfg := ginshow.Default()
	cfg.EnableMiddleware = true
	ginshow.Mount(r, cfg)

	r.GET("/api/sleep/:ms", func(c *gin.Context) {
		ms, _ := strconv.Atoi(c.Param("ms"))
		if ms > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		c.Status(http.StatusOK)
	})

	for _, ms := range []int{1, 2, 3, 4, 5, 10, 20, 30, 40, 50} {
		req := httptest.NewRequest(http.MethodGet, "/api/sleep/"+strconv.Itoa(ms), nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
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

	if payload.Requests.P95LatencyMs <= 0 || payload.Requests.P99LatencyMs <= 0 {
		t.Fatalf("expected positive percentiles, got p95=%v p99=%v",
			payload.Requests.P95LatencyMs, payload.Requests.P99LatencyMs)
	}
	if payload.Requests.P99LatencyMs < payload.Requests.P95LatencyMs {
		t.Fatalf("expected p99 >= p95, got p95=%v p99=%v",
			payload.Requests.P95LatencyMs, payload.Requests.P99LatencyMs)
	}
}
