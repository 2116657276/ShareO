package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler() *AuthHandler { return &AuthHandler{svc: service.NewAuthService()} }

func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.svc.Register(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// Set cookie for web
	http.SetCookie(c.Writer, &http.Cookie{
			Name:     "token",
			Value:    resp.Token,
			Path:     "/",
			MaxAge:   3600 * 72,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	response.Success(c, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.svc.Login(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
			Name:     "token",
			Value:    resp.Token,
			Path:     "/",
			MaxAge:   3600 * 72,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	response.Success(c, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	response.Success(c, nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetInt64("user_id")
	user, err := h.svc.GetProfile(userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req struct {
		AvatarURL string `json:"avatar_url"`
		Bio       string `json:"bio"`
		Email     string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.UpdateProfile(userID, req.AvatarURL, req.Bio, req.Email); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// --- Web page handlers ---

func (h *AuthHandler) LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", userData(c, gin.H{"title": "登录 - ShareO"}))
}

func (h *AuthHandler) RegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", userData(c, gin.H{"title": "注册 - ShareO"}))
}

// handle web form login
func (h *AuthHandler) WebLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	resp, err := h.svc.Login(service.LoginReq{Username: username, Password: password})
	if err != nil {
		c.HTML(http.StatusOK, "login.html", userData(c, gin.H{"title": "登录 - ShareO", "Error": err.Error()}))
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
			Name:     "token",
			Value:    resp.Token,
			Path:     "/",
			MaxAge:   3600 * 72,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	if resp.User.Role == "admin" {
		c.Redirect(http.StatusFound, "/admin")
	} else {
		c.Redirect(http.StatusFound, "/home")
	}
}

func (h *AuthHandler) WebRegister(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	email := c.PostForm("email")
	resp, err := h.svc.Register(service.RegisterReq{Username: username, Password: password, Email: email})
	if err != nil {
		c.HTML(http.StatusOK, "register.html", userData(c, gin.H{"title": "注册 - ShareO", "Error": err.Error()}))
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
			Name:     "token",
			Value:    resp.Token,
			Path:     "/",
			MaxAge:   3600 * 72,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	c.Redirect(http.StatusFound, "/home")
}

func (h *AuthHandler) SettingsPage(c *gin.Context) {
	userID := c.GetInt64("user_id")
	user, _ := h.svc.GetProfile(userID)
	c.HTML(http.StatusOK, "settings.html", userData(c, gin.H{
		"title": "设置 - ShareO",
		"Email": user.Email,
		"Bio":   user.Bio,
	}))
}

func (h *AuthHandler) WebSettings(c *gin.Context) {
	userID := c.GetInt64("user_id")
	email := c.PostForm("email")
	bio := c.PostForm("bio")
	avatarURL := c.PostForm("avatar_url")
	if err := h.svc.UpdateProfile(userID, avatarURL, bio, email); err != nil {
		c.HTML(http.StatusOK, "settings.html", userData(c, gin.H{
			"title": "设置 - ShareO",
			"Email": email, "Bio": bio,
			"Error": "保存失败: " + err.Error(),
		}))
		return
	}
	// Also update email
	user, _ := h.svc.GetProfile(userID)
	c.HTML(http.StatusOK, "settings.html", userData(c, gin.H{
		"title":   "设置 - ShareO",
		"Email":   user.Email,
		"Bio":     user.Bio,
		"Success": "设置已保存",
	}))
}

func (h *AuthHandler) WebLogout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, "/login")
}

