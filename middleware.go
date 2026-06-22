package ginshow

import (
	"log"
	"strings"
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
		if strings.HasPrefix(path, cfg.PprofPrefix) || path == cfg.MetricsPath {
			c.Next()
			return
		}

		globalStats.beginRequest()
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		slow := cfg.SlowRequestThreshold > 0 && duration >= cfg.SlowRequestThreshold
		globalStats.recordRequest(duration, slow)
		globalStats.endRequest()

		if slow {
			log.Printf("[ginshow] slow request: method=%s path=%s status=%d duration=%s client=%s",
				c.Request.Method,
				c.FullPath(),
				c.Writer.Status(),
				duration,
				c.ClientIP(),
			)
		}
	}
}
