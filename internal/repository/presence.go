package repository

import (
	"context"
	"fmt"
	"time"
)

type RedisPresenceStore struct{}

func NewRedisPresenceStore() *RedisPresenceStore { return &RedisPresenceStore{} }

func (s *RedisPresenceStore) SetOnline(ctx context.Context, userID int64, ttl time.Duration) error {
	return RDB.Set(ctx, presenceKey(userID), time.Now().Unix(), ttl).Err()
}

func (s *RedisPresenceStore) SetOffline(ctx context.Context, userID int64) error {
	return RDB.Del(ctx, presenceKey(userID)).Err()
}

func (s *RedisPresenceStore) IsOnline(ctx context.Context, userID int64) (bool, error) {
	count, err := RDB.Exists(ctx, presenceKey(userID)).Result()
	return count > 0, err
}

func presenceKey(userID int64) string { return fmt.Sprintf("ws:online:%d", userID) }
