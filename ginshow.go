package ginshow

import (
	"runtime"

	"github.com/gin-gonic/gin"
)

// Mount registers pprof routes, metrics endpoint, and optional middleware on a Gin engine.
//
// Usage:
//
//	r := gin.Default()
//	ginshow.Mount(r, ginshow.Default())
//	r.Run(":8080")
func Mount(r *gin.Engine, cfg Config) {
	cfg = cfg.withDefaults()
	applyRuntimeProfiling(cfg)

	if cfg.EnableMiddleware {
		r.Use(Middleware(cfg))
	}

	registerDebugRoutes(r, cfg)
}

// Attach registers only debug endpoints on an existing router group.
// Useful when you want middleware on the engine but debug routes under /admin/debug.
func Attach(group *gin.RouterGroup, cfg Config) {
	cfg = cfg.withDefaults()
	applyRuntimeProfiling(cfg)
	registerDebugRoutes(group, cfg)
}

func applyRuntimeProfiling(cfg Config) {
	if cfg.BlockProfileRate > 0 {
		runtime.SetBlockProfileRate(cfg.BlockProfileRate)
	}
	if cfg.MutexProfileFraction > 0 {
		runtime.SetMutexProfileFraction(cfg.MutexProfileFraction)
	}
}
