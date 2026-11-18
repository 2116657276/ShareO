package repository

import (
	"context"
	"errors"
	"testing"
)

func TestLoginCacheOperationsFailClosedWhenRedisIsNotInitialized(t *testing.T) {
	previous := RDB
	RDB = nil
	t.Cleanup(func() { RDB = previous })

	if err := CacheLoginToken(context.Background(), 1, "token", LoginCacheTTL); !errors.Is(err, ErrSessionStoreUnavailable) {
		t.Fatalf("CacheLoginToken error = %v, want ErrSessionStoreUnavailable", err)
	}
	if _, err := GetLoginToken(context.Background(), 1); !errors.Is(err, ErrSessionStoreUnavailable) {
		t.Fatalf("GetLoginToken error = %v, want ErrSessionStoreUnavailable", err)
	}
	if err := DeleteLoginToken(context.Background(), 1); !errors.Is(err, ErrSessionStoreUnavailable) {
		t.Fatalf("DeleteLoginToken error = %v, want ErrSessionStoreUnavailable", err)
	}
	if err := RefreshLoginToken(context.Background(), 1, LoginCacheTTL); !errors.Is(err, ErrSessionStoreUnavailable) {
		t.Fatalf("RefreshLoginToken error = %v, want ErrSessionStoreUnavailable", err)
	}
}
