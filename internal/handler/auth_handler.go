package handler

import (
	"net/http"
	"strconv"

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
	c.SetCookie("token", resp.Token, 3600*72, "/", "", false, true)
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
	c.SetCookie("token", resp.Token, 3600*72, "/", "", false, true)
	response.Success(c, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
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
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.UpdateProfile(userID, req.AvatarURL, req.Bio, ""); err != nil {
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
	c.SetCookie("token", resp.Token, 3600*72, "/", "", false, true)
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
	c.SetCookie("token", resp.Token, 3600*72, "/", "", false, true)
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
	if err := h.svc.UpdateProfile(userID, "", bio, email); err != nil {
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
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func getInt64Param(c *gin.Context, key string) int64 {
	v, _ := strconv.ParseInt(c.Param(key), 10, 64)
	return v
}
