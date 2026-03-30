package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	client *redis.Client
}

func NewRedisLimiter(addr, password string, db int) *RedisLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisLimiter{client: client}
}

func (r *RedisLimiter) Allow(ctx context.Context, clientID, route string, maxRequests int, windowSeconds int) (bool, int, error) {
	key := fmt.Sprintf("ratelimit:%s:%s", clientID, route)
	now := time.Now().UnixMilli()
	windowMs := int64(windowSeconds) * 1000

	pipe := r.client.TxPipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", now-windowMs))
	countCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now),
		Member: fmt.Sprintf("%d", now),
	})
	pipe.Expire(ctx, key, time.Duration(windowSeconds)*time.Second)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("redis pipeline error: %w", err)
	}

	count := int(countCmd.Val())
	remaining := maxRequests - count - 1

	if count >= maxRequests {
		return false, 0, nil
	}

	return true, remaining, nil
}

func (r *RedisLimiter) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}