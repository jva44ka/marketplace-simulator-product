package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	redisContracts "github.com/jva44ka/marketplace-simulator-product/api_internal/redis"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/redis/go-redis/v9"
)

type ProductCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewProductCache(client *redis.Client, ttl time.Duration) *ProductCache {
	return &ProductCache{client: client, ttl: ttl}
}

// Get returns the cached product, or (nil, nil) on a cache miss.
// Any Redis error is returned as-is; callers should treat errors as a miss
// and fall back to the database.
func (c *ProductCache) Get(ctx context.Context, sku uint64) (*models.Product, error) {
	val, err := c.client.Get(ctx, cacheKey(sku)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil // cache miss — not an error
	}
	if err != nil {
		return nil, fmt.Errorf("ProductCache.Get sku=%d: %w", sku, err)
	}

	var product models.Product
	if err = json.Unmarshal(val, &product); err != nil {
		return nil, fmt.Errorf("ProductCache.Get sku=%d: unmarshal: %w", sku, err)
	}
	return &product, nil
}

// Set writes a product to Redis with the configured TTL.
// Errors are logged but not propagated — cache writes are best-effort.
func (c *ProductCache) Set(ctx context.Context, product *models.Product) {
	productRedis := redisContracts.FromModel(product)

	data, err := json.Marshal(&productRedis)
	if err != nil {
		slog.ErrorContext(ctx, "ProductCache.Set: marshal failed", "sku", product.Sku, "err", err)
		return
	}
	if err = c.client.Set(ctx, cacheKey(product.Sku), data, c.ttl).Err(); err != nil {
		slog.WarnContext(ctx, "ProductCache.Set: redis write failed", "sku", product.Sku, "err", err)
	}
}

// Delete removes a product from Redis. Errors are logged but not propagated.
func (c *ProductCache) Delete(ctx context.Context, sku uint64) {
	if err := c.client.Del(ctx, cacheKey(sku)).Err(); err != nil {
		slog.WarnContext(ctx, "ProductCache.Delete: redis delete failed", "sku", sku, "err", err)
	}
}

func cacheKey(sku uint64) string {
	return fmt.Sprintf("product:%d", sku)
}
