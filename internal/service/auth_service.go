package service

import (
	"context"
	"errors"
	"fmt"
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

var ErrAuthSessionUnavailable = errors.New("auth session unavailable")

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrAccountBanned      = errors.New("账号已被封禁")
	ErrUsernameInvalid    = errors.New("用户名包含非法字符")
	ErrUsernameTooShort   = errors.New("用户名至少2个字符")
	ErrUsernameTaken      = errors.New("用户名已存在")
	ErrPasswordTooShort   = errors.New("新密码至少6个字符")
	ErrOriginalPassword   = errors.New("原密码错误")
	ErrUserNotFound       = errors.New("用户不存在")
)

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
	Token string    `json:"token"`
	User  *AuthUser `json:"user"`
}

func validateLoginCandidate(user *model.User) error {
	if user == nil || user.IsBot != 0 {
		return ErrInvalidCredentials
	}
	if user.Status == 0 {
		return ErrAccountBanned
	}
	return nil
}

func (s *AuthService) Register(req RegisterReq) (*AuthResp, error) {
	req.Username = strings.TrimSpace(req.Username)
	// Sanitize: reject HTML/script characters
	if strings.ContainsAny(req.Username, "<>\"'&/\\") {
		return nil, ErrUsernameInvalid
	}
	if len(req.Username) < 2 {
		return nil, ErrUsernameTooShort
	}

	existing, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUsernameTaken
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
		if repository.IsUniqueViolation(err) {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}

	if user.SessionVersion <= 0 {
		user.SessionVersion = 1
	}
	token, err := jwt.GenerateToken(user.ID, user.Username, user.Role, user.SessionVersion)
	if err != nil {
		return nil, err
	}

	// Cache login state in Redis for 30-min sliding window
	if err := repository.CacheLoginToken(context.Background(), user.ID, token, repository.LoginCacheTTL); err != nil {
		if errors.Is(err, repository.ErrSessionStoreUnavailable) {
			return nil, fmt.Errorf("%w: 登录服务暂不可用", ErrAuthSessionUnavailable)
		}
		slog.Warn("failed to cache login token", "user_id", user.ID, "err", err)
	}

	return &AuthResp{Token: token, User: ToAuthUser(user)}, nil
}

func (s *AuthService) Login(req LoginReq) (*AuthResp, error) {
	req.Username = strings.TrimSpace(req.Username)
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if err := validateLoginCandidate(user); err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.SessionVersion <= 0 {
		user.SessionVersion = 1
	}
	token, err := jwt.GenerateToken(user.ID, user.Username, user.Role, user.SessionVersion)
	if err != nil {
		return nil, err
	}

	// Cache login state in Redis for sliding window
	if err := repository.CacheLoginToken(context.Background(), user.ID, token, repository.LoginCacheTTL); err != nil {
		if errors.Is(err, repository.ErrSessionStoreUnavailable) {
			return nil, fmt.Errorf("%w: 登录服务暂不可用", ErrAuthSessionUnavailable)
		}
		slog.Warn("failed to cache login token", "user_id", user.ID, "err", err)
	}

	return &AuthResp{Token: token, User: ToAuthUser(user)}, nil
}

func (s *AuthService) GetProfile(userID int64) (*AuthUser, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	return ToAuthUser(user), nil
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
	return s.userRepo.UpdateFields(userID, map[string]interface{}{
		"email": email,
		"bio":   bio,
	})
}

// ChangePassword validates the old password, updates to the new one, and clears login cache.
func (s *AuthService) ChangePassword(userID int64, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return ErrPasswordTooShort
	}
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrOriginalPassword
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.userRepo.UpdatePasswordAndBumpSession(context.Background(), userID, string(hashed)); err != nil {
		return err
	}
	// Force re-login by deleting login cache
	if err := repository.DeleteLoginToken(context.Background(), userID); err != nil {
		slog.Warn("failed to delete login cache on password change", "user_id", userID, "err", err)
	}
	return nil
}

// RevokeSessions invalidates all previously issued JWTs for the user. Redis
// deletion remains a fast-path cleanup, while the database version is the
// durable revocation source.
func (s *AuthService) RevokeSessions(userID int64) error {
	return s.userRepo.BumpSessionVersion(context.Background(), userID)
}

func IsAuthClientError(err error) bool {
	for _, candidate := range []error{
		ErrInvalidCredentials, ErrAccountBanned, ErrUsernameInvalid, ErrUsernameTooShort, ErrUsernameTaken,
		ErrPasswordTooShort, ErrOriginalPassword, ErrUserNotFound,
	} {
		if errors.Is(err, candidate) {
			return true
		}
	}
	return false
}
