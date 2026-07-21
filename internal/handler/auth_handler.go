package handler

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type AuthHandler struct {
	svc          *service.AuthService
	disconnector interface{ DisconnectUser(int64) }
}

func NewAuthHandler(disconnector ...interface{ DisconnectUser(int64) }) *AuthHandler {
	h := &AuthHandler{svc: service.NewAuthService()}
	if len(disconnector) > 0 {
		h.disconnector = disconnector[0]
	}
	return h
}

func (h *AuthHandler) disconnectUser(userID int64) {
	if h.disconnector != nil && userID > 0 {
		h.disconnector.DisconnectUser(userID)
	}
}

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
		MaxAge:   int(jwt.ExpireDuration().Seconds()),
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
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
		MaxAge:   int(jwt.ExpireDuration().Seconds()),
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
		SameSite: http.SameSiteLaxMode,
	})
	response.Success(c, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Delete login cache to immediately invalidate the session
	userID := c.GetInt64("user_id")
	if userID > 0 {
		if err := repository.DeleteLoginToken(c.Request.Context(), userID); err != nil {
			slog.Warn("failed to delete login cache", "user_id", userID, "err", err)
		}
		h.disconnectUser(userID)
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
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
		MaxAge:   int(jwt.ExpireDuration().Seconds()),
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
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
		MaxAge:   int(jwt.ExpireDuration().Seconds()),
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, "/home")
}

func (h *AuthHandler) SettingsPage(c *gin.Context) {
	userID := c.GetInt64("user_id")
	user, err := h.svc.GetProfile(userID)
	email, bio := "", ""
	if err == nil && user != nil {
		email = user.Email
		bio = user.Bio
	}
	c.HTML(http.StatusOK, "settings.html", userData(c, gin.H{
		"title": "设置 - ShareO",
		"Email": email,
		"Bio":   bio,
	}))
}

func (h *AuthHandler) WebSettings(c *gin.Context) {
	userID := c.GetInt64("user_id")
	email := c.PostForm("email")
	bio := c.PostForm("bio")
	if err := h.svc.UpdateProfileWeb(userID, email, bio); err != nil {
		c.HTML(http.StatusOK, "settings.html", userData(c, gin.H{
			"title": "设置 - ShareO",
			"Email": email, "Bio": bio,
			"Error": "保存失败: " + err.Error(),
		}))
		return
	}
	// Re-fetch profile to show updated values
	user, err := h.svc.GetProfile(userID)
	userEmail, userBio := email, bio
	if err == nil && user != nil {
		userEmail = user.Email
		userBio = user.Bio
	}
	c.HTML(http.StatusOK, "settings.html", userData(c, gin.H{
		"title":   "设置 - ShareO",
		"Email":   userEmail,
		"Bio":     userBio,
		"Success": "设置已保存",
	}))
}

func (h *AuthHandler) WebLogout(c *gin.Context) {
	// Delete login cache to immediately invalidate the session
	userID := c.GetInt64("user_id")
	if userID > 0 {
		if err := repository.DeleteLoginToken(c.Request.Context(), userID); err != nil {
			slog.Warn("failed to delete login cache on web logout", "user_id", userID, "err", err)
		}
		h.disconnectUser(userID)
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, "/login")
}

// ChangePassword handles API password change (JSON).
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.disconnectUser(userID)
	response.Success(c, nil)
}

// WebChangePassword handles Web form password change.
func (h *AuthHandler) WebChangePassword(c *gin.Context) {
	userID := c.GetInt64("user_id")
	oldPassword := c.PostForm("old_password")
	newPassword := c.PostForm("new_password")
	if err := h.svc.ChangePassword(userID, oldPassword, newPassword); err != nil {
		c.HTML(http.StatusOK, "settings.html", userData(c, gin.H{
			"title": "设置 - ShareO",
			"Error": "密码修改失败: " + err.Error(),
		}))
		return
	}
	h.disconnectUser(userID)
	// Force re-login: clear cookie and redirect
	http.SetCookie(c.Writer, &http.Cookie{
		Name: "token", Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: gin.Mode() == gin.ReleaseMode, SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, "/login?message="+url.QueryEscape("密码已修改，请重新登录"))
}
