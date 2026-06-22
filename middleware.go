package ginshow

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Middleware records request latency and optionally logs slow requests.
func Middleware(cfg Config) gin.HandlerFunc {
	cfg = cfg.withDefaults()

	return func(c *gin.Context) {
		if !cfg.EnableMiddleware {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if isInternalDebugPath(path, cfg) {
			c.Next()
			return
		}

		globalStats.beginRequest()
		promBeginRequest()
		start := time.Now()

		var panicVal any
		defer func() {
			duration := time.Since(start)
			method := c.Request.Method
			fullPath := c.FullPath()
			status := c.Writer.Status()
			panicked := false

			if r := recover(); r != nil {
				panicVal = r
				panicked = true
				status = http.StatusInternalServerError
			}

			slow := !panicked && cfg.SlowRequestThreshold > 0 && duration >= cfg.SlowRequestThreshold

			globalStats.recordRequest(method, fullPath, status, duration, slow, panicked)
			promRecordRequest(method, fullPath, status, duration, slow, panicked)
			globalStats.endRequest()
			promEndRequest()

			if slow {
				log.Printf("[ginshow] slow request: method=%s route=%s status=%d duration=%s client=%s",
					method,
					routeKey(method, fullPath),
					status,
					duration,
					c.ClientIP(),
				)
			}

			if panicVal != nil {
				panic(panicVal)
			}
		}()

		c.Next()
	}
}
