package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheClient[T any] struct {
	redisClient *redis.Client
	ttl         time.Duration
}

func NewCacheClient[T any](client *redis.Client, ttl time.Duration) *CacheClient[T] {
	return &CacheClient[T]{redisClient: client, ttl: ttl}
}

func (c *CacheClient[T]) Get(ctx context.Context, key string) (*T, error) {
	value, err := c.redisClient.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil // cache miss — not an error
	}
	if err != nil {
		return nil, err
	}

	var parsedValue T
	if err = json.Unmarshal(value, &parsedValue); err != nil {
		return nil, err
	}
	return &parsedValue, nil
}

func (c *CacheClient[T]) Set(ctx context.Context, key string, value *T) error {
	data, err := json.Marshal(&value)
	if err != nil {
		return err
	}
	if err = c.redisClient.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return err
	}

	return nil
}

func (c *CacheClient[T]) Delete(ctx context.Context, key string) error {
	if err := c.redisClient.Del(ctx, key).Err(); err != nil {
		return err
	}

	return nil
}
