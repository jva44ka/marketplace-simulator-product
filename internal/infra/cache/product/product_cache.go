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

// CacheMetricsReporter is satisfied by *metrics.CacheMetrics via structural typing.
//
// operation: "get" | "set" | "delete"
// result for get:    "hit" | "miss" | "stale" | "error"
// result for set:    "success" | "error"
// result for delete: "success" | "error"
type CacheMetricsReporter interface {
	ReportOperation(operation, result string, duration time.Duration)
}

type ProductCache struct {
	client  *cache.CacheClient[redisContracts.Product]
	ttl     time.Duration
	metrics CacheMetricsReporter // nil → metrics disabled
}

func NewProductCache(redisClient *redis.Client, ttl time.Duration, m CacheMetricsReporter) *ProductCache {
	return &ProductCache{
		client:  cache.NewCacheClient[redisContracts.Product](redisClient, ttl),
		ttl:     ttl,
		metrics: m,
	}
}

func (c *ProductCache) Get(ctx context.Context, sku uint64) (*models.Product, error) {
	start := time.Now()
	product, err := c.client.Get(ctx, cacheKey(sku))
	duration := time.Since(start)

	if err != nil {
		c.reportOp("get", "error", duration)
		return nil, fmt.Errorf("ProductCache.Get sku=%d: %w", sku, err)
	}

	if product == nil {
		c.reportOp("get", "miss", duration)
		return nil, nil
	}

	c.reportOp("get", "hit", duration)
	return &models.Product{
		Sku:           product.Sku,
		Price:         product.Price,
		Name:          product.Name,
		Count:         product.Count,
		ReservedCount: product.ReservedCount,
		TransactionId: product.TransactionId,
	}, nil
}

// Set не возвращает error т.к. запускается асинхронно в отдельной горутине без ожидания результата.
func (c *ProductCache) Set(ctx context.Context, product *models.Product) {
	start := time.Now()
	productRedis := redisContracts.FromModel(product)

	if err := c.client.Set(ctx, cacheKey(product.Sku), productRedis); err != nil {
		c.reportOp("set", "error", time.Since(start))
		slog.WarnContext(ctx, "ProductCache.Set: redis write failed", "sku", product.Sku, "err", err)
		return
	}
	c.reportOp("set", "success", time.Since(start))
}

func (c *ProductCache) Delete(ctx context.Context, sku uint64) {
	start := time.Now()
	if err := c.client.Delete(ctx, cacheKey(sku)); err != nil {
		c.reportOp("delete", "error", time.Since(start))
		slog.WarnContext(ctx, "ProductCache.Delete: redis delete failed", "sku", sku, "err", err)
		return
	}
	c.reportOp("delete", "success", time.Since(start))
}

func (c *ProductCache) reportOp(op, result string, d time.Duration) {
	if c.metrics != nil {
		c.metrics.ReportOperation(op, result, d)
	}
}

func cacheKey(sku uint64) string {
	return fmt.Sprintf("product:%d", sku)
}
