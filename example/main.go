package main

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
)

func main() {
	r := gin.Default()

	// One-line integration: pprof + runtime metrics + request monitoring.
	ginshow.Mount(r, ginshow.Default())

	r.GET("/api/hello", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.JSON(200, gin.H{"message": "hello"})
	})

	// Production example:
	// ginshow.Mount(r, ginshow.Production("admin", "your-secret"))

	r.Run(":8080")
}
