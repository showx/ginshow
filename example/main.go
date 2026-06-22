package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
)

func main() {
	// 账号密码可通过环境变量覆盖，生产环境务必修改默认值。
	username := envOr("GINSHOW_USER", "admin")
	password := envOr("GINSHOW_PASS", "ginshow")

	r := gin.Default()

	// 启用 Basic Auth：面板需登录，metrics / pprof / 火焰图 API 亦受保护。
	cfg := ginshow.Production(username, password)
	ginshow.Mount(r, cfg)

	r.GET("/api/hello", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.JSON(200, gin.H{"message": "hello"})
	})

	log.Println("ginshow example started")
	log.Printf("  dashboard : http://localhost:8080%s", cfg.DashboardPath)
	log.Printf("  username  : %s", username)
	log.Printf("  password  : %s", password)

	r.Run(":8080")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
