package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo *repository.UserRepo
}

func NewAuthService() *AuthService {
	return &AuthService{userRepo: repository.NewUserRepo()}
}

type RegisterReq struct {
	Username string `json:"username" binding:"required,min=2,max=50"`
	Password string `json:"password" binding:"required,min=6,max=100"`
	Email    string `json:"email" binding:"omitempty,email"`
}

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResp struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

func (s *AuthService) Register(req RegisterReq) (*AuthResp, error) {
	req.Username = strings.TrimSpace(req.Username)
	// Sanitize: reject HTML/script characters
	if strings.ContainsAny(req.Username, "<>\"'&/\\") {
		return nil, errors.New("用户名包含非法字符")
	}
	if len(req.Username) < 2 {
		return nil, errors.New("用户名至少2个字符")
	}

	existing, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("用户名已存在")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:     req.Username,
		PasswordHash: string(hashed),
		Email:        req.Email,
		Role:         model.RoleUser,
		Status:       model.UserStatusActive,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	token, err := jwt.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	// Cache login state in Redis for 30-min sliding window
	if err := repository.CacheLoginToken(context.Background(), user.ID, token, repository.LoginCacheTTL); err != nil {
		slog.Warn("failed to cache login token", "user_id", user.ID, "err", err)
	}

	return &AuthResp{Token: token, User: user}, nil
}

func (s *AuthService) Login(req LoginReq) (*AuthResp, error) {
	req.Username = strings.TrimSpace(req.Username)
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("用户名或密码错误")
	}
	if user.Status == 0 {
		return nil, errors.New("账号已被封禁")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	token, err := jwt.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	// Cache login state in Redis for sliding window
	if err := repository.CacheLoginToken(context.Background(), user.ID, token, repository.LoginCacheTTL); err != nil {
		slog.Warn("failed to cache login token", "user_id", user.ID, "err", err)
	}

	return &AuthResp{Token: token, User: user}, nil
}

func (s *AuthService) GetProfile(userID int64) (*model.User, error) {
	return s.userRepo.FindByID(userID)
}

func (s *AuthService) UpdateProfile(userID int64, avatarURL, bio, email string) error {
	// Always update all fields — empty strings clear the field
	updates := map[string]interface{}{
		"avatar_url": avatarURL,
		"bio":        bio,
		"email":      email,
	}
	return s.userRepo.UpdateFields(userID, updates)
}

// UpdateProfileWeb only updates fields submitted via the web settings form (email, bio).
// It does NOT touch avatar_url, preventing the form from clearing API-uploaded avatars.
func (s *AuthService) UpdateProfileWeb(userID int64, email, bio string) error {
	updates := map[string]interface{}{}
	if email != "" {
		updates["email"] = email
	}
	if bio != "" {
		updates["bio"] = bio
	}
	if len(updates) == 0 {
		return nil
	}
	return s.userRepo.UpdateFields(userID, updates)
}

// ChangePassword validates the old password, updates to the new one, and clears login cache.
func (s *AuthService) ChangePassword(userID int64, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("新密码至少6个字符")
	}
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("用户不存在")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("原密码错误")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.userRepo.UpdateFields(userID, map[string]interface{}{"password_hash": string(hashed)}); err != nil {
		return err
	}
	// Force re-login by deleting login cache
	if err := repository.DeleteLoginToken(context.Background(), userID); err != nil {
		slog.Warn("failed to delete login cache on password change", "user_id", userID, "err", err)
	}
	return nil
}
