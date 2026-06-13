package router

import (
	"net/http"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/handler"
	"github.com/zhoujianlin/ShareO/internal/middleware"
	jwtpkg "github.com/zhoujianlin/ShareO/internal/pkg/jwt"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	r.Use(middleware.NoCache())
	r.Use(gzip.Gzip(gzip.DefaultCompression)) // gzip压缩，大幅减少HTML/JSON/CSS传输
	r.Static("/static", "web/static")

	authH := handler.NewAuthHandler()
	postH := handler.NewPostHandler()
	feedH := handler.NewFeedHandler()
	socialH := handler.NewSocialHandler()
	adminH := handler.NewAdminHandler()
	userH := handler.NewUserHandler()
	uploadH := handler.NewUploadHandler()
	notifH := handler.NewNotificationHandler()
	topicH := handler.NewTopicHandler()

	// === 首页：根据登录态分流 ===
	// 未登录 → 登录页(区分admin/user入口)
	// 已登录admin → /admin
	// 已登录user → /home
	r.GET("/", func(c *gin.Context) {
		token, err := c.Cookie("token")
		if err != nil || token == "" {
			authH.LoginPage(c)
			return
		}
		claims, err := jwtpkg.ParseToken(token)
		if err != nil {
			authH.LoginPage(c)
			return
		}
		if claims.Role == "admin" {
			c.Redirect(http.StatusFound, "/admin")
		} else {
			c.Redirect(http.StatusFound, "/home")
		}
	})

	// === Public API ===
	api := r.Group("/api/v1")
	{
		// Rate limit auth endpoints: 10 requests per minute per IP
		authLimiter := middleware.RateLimit(10, 1*time.Minute)
		api.POST("/auth/register", authLimiter, authH.Register)
		api.POST("/auth/login", authLimiter, authH.Login)

		pub := api.Group("")
		pub.Use(middleware.OptionalAuth())
		{
			pub.GET("/feed", feedH.GetFeed)
			pub.GET("/search", feedH.Search)
			pub.GET("/posts/:id", postH.GetByID)
			pub.GET("/posts/:id/comments", socialH.GetComments)
			pub.GET("/users/:id/following", socialH.GetFollowing)
			pub.GET("/users/:id/followers", socialH.GetFollowers)
		}
	}

	// === Authenticated API ===
	authAPI := api.Group("")
	authAPI.Use(middleware.AuthRequired())
	{
		// Rate limit write operations
		postLimiter := middleware.RateLimit(30, 1*time.Minute)
		uploadLimiter := middleware.RateLimit(20, 1*time.Minute)

		authAPI.GET("/auth/me", authH.Me)
		authAPI.POST("/auth/logout", authH.Logout)
		authAPI.PUT("/auth/profile", authH.UpdateProfile)
		authAPI.POST("/posts", postLimiter, postH.Create)
		authAPI.PUT("/posts/:id", postH.Update)
		authAPI.DELETE("/posts/:id", postH.Delete)
		authAPI.POST("/posts/:id/repost", postH.Repost)
		authAPI.POST("/posts/:id/like", socialH.ToggleLike)
		authAPI.POST("/posts/:id/favorite", socialH.ToggleFavorite)
		authAPI.GET("/favorites", socialH.GetFavorites)
		authAPI.GET("/likes", socialH.GetLikes)
		authAPI.POST("/posts/:id/comments", socialH.CreateComment)
		authAPI.DELETE("/comments/:cid", socialH.DeleteComment)
		authAPI.POST("/users/:id/follow", socialH.ToggleFollow)
		authAPI.POST("/upload", uploadLimiter, uploadH.UploadImage)

		// Notifications
		authAPI.GET("/notifications", notifH.List)
		authAPI.PUT("/notifications/:id/read", notifH.MarkRead)
		authAPI.PUT("/notifications/read-all", notifH.MarkAllRead)
		authAPI.GET("/notifications/unread-count", notifH.UnreadCount)
	}

	api.Any("/images/*objectName", uploadH.ServeImage)

	// === Admin API ===
	adminAPI := api.Group("/admin")
	adminAPI.Use(middleware.AuthRequired(), middleware.AdminRequired())
	{
		adminAPI.GET("/stats", adminH.GetStats)
		adminAPI.GET("/pending-posts", adminH.GetPendingPosts)
		adminAPI.DELETE("/posts/:id", adminH.DeletePost)
		adminAPI.POST("/posts/:id/approve", adminH.ApprovePost)
		adminAPI.POST("/posts/:id/reject", adminH.RejectPost)
		adminAPI.GET("/users", adminH.GetUsers)
		adminAPI.PUT("/users/:id/status", adminH.UpdateUserStatus)
		adminAPI.GET("/logs", adminH.GetLogs)
	}

	// === Web Pages (Public) ===
	r.GET("/login", authH.LoginPage)
	r.GET("/register", authH.RegisterPage)
	r.POST("/login", authH.WebLogin)
	r.POST("/register", authH.WebRegister)

	// === Web Pages (Auth required) ===
	needLogin := r.Group("")
	needLogin.Use(middleware.AuthRequired())
	{
		needLogin.GET("/home", feedH.HomePage)
		needLogin.GET("/post/:id", postH.DetailPage)
		needLogin.GET("/post/create", postH.CreatePage)
		needLogin.GET("/post/:id/edit", postH.EditPage)
		needLogin.POST("/post/create", postH.WebCreate)
		needLogin.POST("/post/:id/edit", postH.WebUpdate)
		needLogin.POST("/post/:id/comment", socialH.WebCreateComment)
		needLogin.GET("/user/:id", userH.ProfilePage)
		needLogin.GET("/settings", authH.SettingsPage)
		needLogin.POST("/settings", authH.WebSettings)
		needLogin.GET("/notifications", notifH.NotificationsPage)
		needLogin.GET("/topic/:id", topicH.TopicPage)
		needLogin.GET("/logout", authH.WebLogout)
	}

	// === Admin Web Pages ===
	adminWeb := r.Group("/admin")
	adminWeb.Use(middleware.AuthRequired(), middleware.AdminRequired())
	{
		adminWeb.GET("/", adminH.Dashboard)
		adminWeb.GET("/review", adminH.Review)
		adminWeb.GET("/users", adminH.UsersPage)
		adminWeb.GET("/logs", adminH.LogsPage)
	}

	return r
}
