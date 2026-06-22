package ginshow

import (
	"net/http/pprof"

	"github.com/gin-gonic/gin"
)

func registerPprof(group *gin.RouterGroup) {
	group.GET("/", gin.WrapF(pprof.Index))
	group.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	group.GET("/profile", gin.WrapF(pprof.Profile))
	group.GET("/symbol", gin.WrapF(pprof.Symbol))
	group.GET("/trace", gin.WrapF(pprof.Trace))
	group.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
	group.GET("/block", gin.WrapH(pprof.Handler("block")))
	group.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	group.GET("/heap", gin.WrapH(pprof.Handler("heap")))
	group.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
	group.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
	group.GET("/flame", flameHandler())
}

func authMiddleware(auth *AuthConfig) gin.HandlerFunc {
	return gin.BasicAuth(gin.Accounts{
		auth.Username: auth.Password,
	})
}

func debugGroup(r gin.IRouter, cfg Config) *gin.RouterGroup {
	if auth := cfg.Auth; auth != nil && auth.Username != "" {
		return r.Group(cfg.PprofPrefix, authMiddleware(auth))
	}
	return r.Group(cfg.PprofPrefix)
}

func registerDebugRoutes(r gin.IRouter, cfg Config) {
	hasAuth := cfg.Auth != nil && cfg.Auth.Username != ""

	if cfg.EnablePprof {
		registerPprof(debugGroup(r, cfg))
	}

	if cfg.EnableMetrics {
		registerGET(r, cfg.MetricsPath, hasAuth, cfg.Auth, metricsHandler())
	}

	if cfg.EnableDashboard {
		registerGET(r, cfg.DashboardPath, hasAuth, cfg.Auth, dashboardHandler(cfg))
	}
}

func registerGET(r gin.IRouter, path string, hasAuth bool, auth *AuthConfig, handler gin.HandlerFunc) {
	if hasAuth {
		r.GET(path, authMiddleware(auth), handler)
		return
	}
	r.GET(path, handler)
}
