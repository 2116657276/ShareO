package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/config"
	"github.com/zhoujianlin/ShareO/internal/middleware"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
	"github.com/zhoujianlin/ShareO/internal/pkg/upload"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"github.com/zhoujianlin/ShareO/internal/router"
)

var (
	mediumToThumbRE = regexp.MustCompile(`/posts/medium/`)
	anyToMediumRE   = regexp.MustCompile(`/posts/(original|thumb)/`)
)

func main() {
	// Load config
	configPath := os.Getenv("SHAREO_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Init structured logger
	var slogHandler slog.Handler
	if cfg.Server.Mode == "release" {
		slogHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		slogHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(slogHandler))

	// Init MySQL
	if err := initWithRetry("MySQL", func() error { return repository.InitDB(cfg.Database, cfg.Server.Mode) }); err != nil {
		log.Fatalf("Failed to init MySQL: %v", err)
	}

	// Init Redis
	if err := initWithRetry("Redis", func() error { return repository.InitRedis(cfg.Redis) }); err != nil {
		log.Fatalf("Failed to init Redis: %v", err)
	}

	// Init MinIO
	if err := initWithRetry("MinIO", func() error { return upload.Init(cfg.MinIO) }); err != nil {
		log.Fatalf("Failed to init MinIO: %v", err)
	}

	// Init JWT
	jwt.Init(cfg.JWT.Secret, cfg.JWT.ExpireHours)

	// Init login cache TTL
	repository.InitLoginCache(cfg.JWT.LoginCacheTTLMin)

	// Setup router with custom functions
	r := router.SetupRouter(cfg.Server.TrustedOrigins...)
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
	})

	// Collect all HTML templates (Go filepath.Glob does not support **).
	rootFiles, _ := filepath.Glob("web/templates/*.html")
	subFiles, _ := filepath.Glob("web/templates/*/*.html")
	allTemplates := append(rootFiles, subFiles...)
	if len(allTemplates) == 0 {
		log.Fatal("no templates found in web/templates/ — check working directory")
	}

	// Reload templates after setting func map
	r.LoadHTMLFiles(allTemplates...)
	protectedHandler, err := middleware.CrossOriginProtection(r, cfg.Server.TrustedOrigins)
	if err != nil {
		log.Fatalf("Failed to configure cross-origin protection: %v", err)
	}

	// Health check
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "ShareO"})
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	slog.Info("ShareO server starting", "addr", addr, "mode", cfg.Server.Mode)
	slog.Info("MinIO bucket configured", "bucket", cfg.MinIO.Bucket)

	// Print helpful startup info
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  ShareO - 拍摄与作品管理系统")
	fmt.Println("========================================")
	fmt.Printf("  Web:     http://localhost%s\n", addr)
	fmt.Println("  管理员账号：由本机初始化配置提供")
	fmt.Printf("  用户:    注册后登录\n")
	fmt.Println("========================================")

	// Graceful shutdown with signal handling
	srv := &http.Server{
		Addr:    addr,
		Handler: protectedHandler,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	slog.Info("server exited")
}

func initWithRetry(name string, initialize func() error) error {
	deadline := time.Now().Add(45 * time.Second)
	delay := 500 * time.Millisecond
	var lastErr error
	for time.Now().Before(deadline) {
		if err := initialize(); err == nil {
			return nil
		} else {
			lastErr = err
			slog.Warn("dependency not ready; retrying", "dependency", name, "delay", delay, "err", err)
		}
		time.Sleep(delay)
		if delay < 3*time.Second {
			delay *= 2
			if delay > 3*time.Second {
				delay = 3 * time.Second
			}
		}
	}
	return fmt.Errorf("%s did not become ready: %w", name, lastErr)
}
