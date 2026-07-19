package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// LoginCacheTTL is the TTL for login state cache entries.
// Set via InitLoginCache after config loading.
var LoginCacheTTL = 30 * time.Minute

// InitLoginCache sets the login cache TTL from config.
func InitLoginCache(ttlMin int) {
	if ttlMin > 0 {
		LoginCacheTTL = time.Duration(ttlMin) * time.Minute
	}
}

// keyPrefix for login state cache keys.
const loginCacheKeyPrefix = "auth:login:"

func loginCacheKey(userID int64) string {
	return loginCacheKeyPrefix + strconv.FormatInt(userID, 10)
}

// CacheLoginToken stores the JWT token for a user in Redis with the given TTL.
// This enables server-side login state tracking and allows forced logout.
func CacheLoginToken(ctx context.Context, userID int64, token string, ttl time.Duration) error {
	if RDB == nil {
		return fmt.Errorf("redis not initialized")
	}
	return RDB.Set(ctx, loginCacheKey(userID), token, ttl).Err()
}

// GetLoginToken retrieves the cached JWT token for a user.
// Returns ("", nil) if the key does not exist (login expired).
func GetLoginToken(ctx context.Context, userID int64) (string, error) {
	if RDB == nil {
		return "", fmt.Errorf("redis not initialized")
	}
	val, err := RDB.Get(ctx, loginCacheKey(userID)).Result()
	if err != nil {
		return "", err // redis.Nil means key not found
	}
	return val, nil
}

// DeleteLoginToken removes the login cache entry, effectively logging the user out.
func DeleteLoginToken(ctx context.Context, userID int64) error {
	if RDB == nil {
		return fmt.Errorf("redis not initialized")
	}
	return RDB.Del(ctx, loginCacheKey(userID)).Err()
}

// RefreshLoginToken extends the TTL of an existing login cache entry.
func RefreshLoginToken(ctx context.Context, userID int64, ttl time.Duration) error {
	if RDB == nil {
		return fmt.Errorf("redis not initialized")
	}
	return RDB.Expire(ctx, loginCacheKey(userID), ttl).Err()
}
