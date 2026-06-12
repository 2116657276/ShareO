package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/config"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
	"github.com/zhoujianlin/ShareO/internal/pkg/upload"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"github.com/zhoujianlin/ShareO/internal/router"
)

func main() {
	// Load config
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Init MySQL
	if err := repository.InitDB(cfg.Database); err != nil {
		log.Fatalf("Failed to init MySQL: %v", err)
	}

	// Init Redis
	if err := repository.InitRedis(cfg.Redis); err != nil {
		log.Fatalf("Failed to init Redis: %v", err)
	}

	// Init MinIO
	if err := upload.Init(cfg.MinIO); err != nil {
		log.Fatalf("Failed to init MinIO: %v", err)
	}

	// Init JWT
	jwt.Init(cfg.JWT.Secret, cfg.JWT.ExpireHours)

	// Setup router with custom functions
	r := router.SetupRouter()
	r.SetFuncMap(template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
		"iterate": func(n int) []int {
			result := make([]int, n)
			for i := 0; i < n; i++ {
				result[i] = i + 1
			}
			return result
		},
		"or": func(a, b string) string {
			if a != "" {
				return a
			}
			return b
		},
	})

	// Reload templates after setting func map
	r.LoadHTMLGlob("web/templates/**/*.html")

	// Health check
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "ShareO"})
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("ShareO server starting on http://localhost%s", addr)
	log.Printf("Mode: %s", cfg.Server.Mode)
	log.Printf("MinIO bucket: %s", cfg.MinIO.Bucket)

	// Print helpful startup info
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  ShareO - 拍摄与作品管理系统")
	fmt.Println("========================================")
	fmt.Printf("  Web:     http://localhost%s\n", addr)
	fmt.Printf("  管理员:  admin / admin123\n")
	fmt.Printf("  用户:    注册后登录\n")
	fmt.Println("========================================")

	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
