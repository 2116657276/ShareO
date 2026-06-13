package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/config"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
	"github.com/zhoujianlin/ShareO/internal/pkg/upload"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"github.com/zhoujianlin/ShareO/internal/router"
)

var hashtagLinkRE = regexp.MustCompile(`#([\p{L}\p{N}_]+)`)
var (
	mediumToThumbRE   = regexp.MustCompile(`/posts/medium/`)
	anyToMediumRE     = regexp.MustCompile(`/posts/(original|thumb)/`)
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
	if err := repository.InitDB(cfg.Database, cfg.Server.Mode); err != nil {
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
		"thumbURL": func(url string) string {
			return mediumToThumbRE.ReplaceAllString(url, "/posts/thumb/")
		},
		"mediumURL": func(url string) string {
			// Ensure URL uses medium size: /posts/original/ → /posts/medium/, /posts/thumb/ → /posts/medium/
			return anyToMediumRE.ReplaceAllString(url, "/posts/medium/")
		},
		"renderHashtags": func(content string) template.HTML {
			safe := hashtagLinkRE.ReplaceAllStringFunc(content, func(match string) string {
				name := strings.TrimPrefix(match, "#")
				return `<a href="/topic/` + template.HTMLEscapeString(name) + `" class="hashtag-link">#` + template.HTMLEscapeString(name) + `</a>`
			})
			return template.HTML(safe)
		},
	})

	// Collect all HTML templates (Go filepath.Glob does not support **).
	rootFiles, _ := filepath.Glob("web/templates/*.html")
	subFiles, _ := filepath.Glob("web/templates/*/*.html")
	allTemplates := append(rootFiles, subFiles...)

	// Reload templates after setting func map
	r.LoadHTMLFiles(allTemplates...)

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

	// Graceful shutdown with signal handling
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}
