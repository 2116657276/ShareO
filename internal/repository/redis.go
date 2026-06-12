package repository

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/zhoujianlin/ShareO/internal/config"
)

var RDB *redis.Client

func InitRedis(cfg config.RedisConfig) error {
	RDB = redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx := context.Background()
	if err := RDB.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect redis: %w", err)
	}

	fmt.Println("Redis connected successfully")
	return nil
}
