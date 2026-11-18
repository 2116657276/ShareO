package repository

import (
	"context"
	"testing"
)

func TestPostRepoIsApprovedImageNilDBReturnsError(t *testing.T) {
	previous := DB
	DB = nil
	t.Cleanup(func() { DB = previous })

	allowed, err := NewPostRepo().IsApprovedImage(context.Background(), "posts/medium/2026/08/12/test.jpg")
	if err == nil || allowed {
		t.Fatalf("IsApprovedImage() = (%v, %v), want (false, error)", allowed, err)
	}
}

func TestUserRepoGetSessionVersionNilDBReturnsError(t *testing.T) {
	previous := DB
	DB = nil
	t.Cleanup(func() { DB = previous })

	version, err := NewUserRepo().GetSessionVersion(context.Background(), 1)
	if err == nil || version != 0 {
		t.Fatalf("GetSessionVersion() = (%d, %v), want (0, error)", version, err)
	}
}
