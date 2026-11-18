package handler

import (
	"errors"
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
		response.BadRequest(c, "请求参数格式错误")
		return
	}
	resp, err := h.svc.Register(req)
	if err != nil {
		if errors.Is(err, service.ErrAuthSessionUnavailable) {
			response.Error(c, http.StatusServiceUnavailable, response.ErrCodeInternal, "登录服务暂不可用，请稍后重试")
			return
		}
		if service.IsAuthClientError(err) {
			response.BadRequest(c, err.Error())
		} else {
			slog.Warn("registration failed", "err", err)
			response.InternalError(c, "注册暂时无法完成，请稍后重试")
		}
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
		response.BadRequest(c, "请求参数格式错误")
		return
	}
	resp, err := h.svc.Login(req)
	if err != nil {
		if errors.Is(err, service.ErrAuthSessionUnavailable) {
			response.Error(c, http.StatusServiceUnavailable, response.ErrCodeInternal, "登录服务暂不可用，请稍后重试")
			return
		}
		if service.IsAuthClientError(err) {
			response.BadRequest(c, err.Error())
		} else {
			slog.Warn("login failed", "err", err)
			response.InternalError(c, "登录暂时无法完成，请稍后重试")
		}
		return
	}
	// Replacing the Redis session must also revoke old live sockets; otherwise
	// an old browser tab can continue receiving private messages.
	h.disconnectUser(resp.User.ID)
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
		if err := h.svc.RevokeSessions(userID); err != nil {
			slog.Warn("failed to persist logout revocation", "user_id", userID, "err", err)
			response.InternalError(c, "退出登录暂时无法完成，请稍后重试")
			return
		}
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
		slog.Warn("profile lookup failed", "user_id", userID, "err", err)
		response.InternalError(c, "个人资料暂时无法加载，请稍后重试")
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
		response.BadRequest(c, "请求参数格式错误")
		return
	}
	if err := h.svc.UpdateProfile(userID, req.AvatarURL, req.Bio, req.Email); err != nil {
		slog.Warn("profile update failed", "user_id", userID, "err", err)
		response.InternalError(c, "个人资料暂时无法保存，请稍后重试")
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
		status := http.StatusOK
		if errors.Is(err, service.ErrAuthSessionUnavailable) {
			status = http.StatusServiceUnavailable
		}
		message := "登录暂时无法完成，请稍后重试"
		if service.IsAuthClientError(err) {
			message = err.Error()
		} else {
			slog.Warn("web login failed", "err", err)
		}
		c.HTML(status, "login.html", userData(c, gin.H{"title": "登录 - ShareO", "Error": message}))
		return
	}
	h.disconnectUser(resp.User.ID)
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
		status := http.StatusOK
		if errors.Is(err, service.ErrAuthSessionUnavailable) {
			status = http.StatusServiceUnavailable
		}
		message := "注册暂时无法完成，请稍后重试"
		if service.IsAuthClientError(err) {
			message = err.Error()
		} else {
			slog.Warn("web registration failed", "err", err)
		}
		c.HTML(status, "register.html", userData(c, gin.H{"title": "注册 - ShareO", "Error": message}))
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
		slog.Warn("web profile update failed", "user_id", userID, "err", err)
		c.HTML(http.StatusOK, "settings.html", userData(c, gin.H{
			"title": "设置 - ShareO",
			"Email": email, "Bio": bio,
			"Error": "保存失败，请稍后重试",
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
		if err := h.svc.RevokeSessions(userID); err != nil {
			slog.Warn("failed to persist web logout revocation", "user_id", userID, "err", err)
			c.HTML(http.StatusServiceUnavailable, "settings.html", userData(c, gin.H{
				"title": "设置 - ShareO", "Error": "退出登录暂时无法完成，请稍后重试",
			}))
			return
		}
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
		response.BadRequest(c, "请求参数格式错误")
		return
	}
	if err := h.svc.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		if service.IsAuthClientError(err) {
			response.BadRequest(c, err.Error())
		} else {
			slog.Warn("password change failed", "user_id", userID, "err", err)
			response.InternalError(c, "密码暂时无法修改，请稍后重试")
		}
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
		message := "密码暂时无法修改，请稍后重试"
		if service.IsAuthClientError(err) {
			message = "密码修改失败: " + err.Error()
		} else {
			slog.Warn("web password change failed", "user_id", userID, "err", err)
		}
		c.HTML(http.StatusOK, "settings.html", userData(c, gin.H{
			"title": "设置 - ShareO",
			"Error": message,
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
