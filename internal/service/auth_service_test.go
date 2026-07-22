package service

import (
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
