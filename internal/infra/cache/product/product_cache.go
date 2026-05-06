package product

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	redisContracts "github.com/jva44ka/marketplace-simulator-product/api_internal/redis"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/cache"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/redis/go-redis/v9"
)

type ProductCache struct {
	client *cache.CacheClient[redisContracts.Product]
	ttl    time.Duration
}

func NewProductCache(redisClient *redis.Client, ttl time.Duration) *ProductCache {
	return &ProductCache{
		client: cache.NewCacheClient[redisContracts.Product](redisClient, ttl),
		ttl:    ttl,
	}
}

func (c *ProductCache) Get(ctx context.Context, sku uint64) (*models.Product, error) {
	product, err := c.client.Get(ctx, cacheKey(sku))
	if err != nil {
		return nil, fmt.Errorf("ProductCache.Get sku=%d: %w", sku, err)
	}

	if product == nil {
		return nil, nil
	}

	return &models.Product{
		Sku:           product.Sku,
		Price:         product.Price,
		Name:          product.Name,
		Count:         product.Count,
		ReservedCount: product.ReservedCount,
		TransactionId: product.TransactionId,
	}, nil
}

// Не возвращает error т.к. запускается асинхронно в отдельной горутине без ожидания результата
// Ошибку некому обрабатывать
func (c *ProductCache) Set(ctx context.Context, product *models.Product) {
	productRedis := redisContracts.FromModel(product)

	if err := c.client.Set(ctx, cacheKey(product.Sku), productRedis); err != nil {
		slog.WarnContext(ctx, "ProductCache.Set: redis write failed", "sku", product.Sku, "err", err)
	}
}

func (c *ProductCache) Delete(ctx context.Context, sku uint64) {
	if err := c.client.Delete(ctx, cacheKey(sku)); err != nil {
		slog.WarnContext(ctx, "ProductCache.Delete: redis delete failed", "sku", sku, "err", err)
	}
}

func cacheKey(sku uint64) string {
	return fmt.Sprintf("product:%d", sku)
}
