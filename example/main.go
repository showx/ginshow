package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
	gshowgorm "github.com/showx/ginshow/gorm"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	username := envOr("GINSHOW_USER", "admin")
	password := envOr("GINSHOW_PASS", "ginshow")

	db, err := gorm.Open(sqlite.Open("file:ginshow_example?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		log.Fatalf("open gorm db: %v", err)
	}

	r := gin.Default()

	cfg := ginshow.Production(username, password)
	cfg.Health.Checks = []ginshow.NamedCheck{
		gshowgorm.Check("primary", db),
	}
	ginshow.Mount(r, cfg)

	r.GET("/api/hello", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.JSON(200, gin.H{"message": "hello"})
	})

	log.Println("ginshow example started")
	log.Printf("  dashboard : http://localhost:8080%s", cfg.DashboardPath)
	log.Printf("  healthz   : http://localhost:8080%s", cfg.Health.HealthzPath)
	log.Printf("  readyz    : http://localhost:8080%s", cfg.Health.ReadyzPath)
	log.Printf("  prometheus: http://localhost:8080%s", cfg.Prometheus.Path)
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
