package service

import (
	"testing"

	"github.com/zhoujianlin/ShareO/internal/repository"
)

func TestFeedServiceGetCachedFeedNilRedisReturnsMiss(t *testing.T) {
	previous := repository.RDB
	repository.RDB = nil
	t.Cleanup(func() { repository.RDB = previous })

	svc := NewFeedService()
	posts, total, hit := svc.getCachedFeed()
	if hit || total != 0 || posts != nil {
		t.Fatalf("getCachedFeed() = (%v, %d, %v), want (nil, 0, false)", posts, total, hit)
	}
}
