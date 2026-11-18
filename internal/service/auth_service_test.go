package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/zhoujianlin/ShareO/internal/model"
)

func TestBotLoginCandidateIsRejected(t *testing.T) {
	err := validateLoginCandidate(&model.User{
		Username: "shareo_bot",
		Status:   model.UserStatusActive,
		IsBot:    1,
	})
	if err == nil || err.Error() != "用户名或密码错误" {
		t.Fatalf("bot login error=%v", err)
	}
}

func TestAuthClientErrorsAreStable(t *testing.T) {
	for _, err := range []error{ErrInvalidCredentials, ErrAccountBanned, ErrUsernameTaken, ErrUsernameTooShort, ErrPasswordTooShort, ErrOriginalPassword} {
		if !IsAuthClientError(err) {
			t.Fatalf("%v should be safe for client mapping", err)
		}
	}
	if IsAuthClientError(errors.New("database constraint detail")) {
		t.Fatal("database details must not be classified as client errors")
	}
}

func TestToAuthUserOmitsPersistenceFieldsAndAssociations(t *testing.T) {
	user := &model.User{
		ID: 7, Username: "portfolio", PasswordHash: "secret-hash",
		Email: "portfolio@example.com", Role: model.RoleUser, Status: model.UserStatusActive,
		IsBot: 0, Posts: []model.Post{{ID: 99}},
	}
	private := ToAuthUser(user)
	encoded, err := json.Marshal(private)
	if err != nil {
		t.Fatalf("marshal auth user: %v", err)
	}
	body := string(encoded)
	for _, forbidden := range []string{"password_hash", "secret-hash", "status", "is_bot", "posts", "updated_at"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("auth user contains persistence field %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"username":"portfolio"`) {
		t.Fatalf("auth user lost username: %s", body)
	}
}
