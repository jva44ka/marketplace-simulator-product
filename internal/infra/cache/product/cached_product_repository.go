package product

import (
	"context"
	"log/slog"

	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type productDbRepository interface {
	GetBySku(ctx context.Context, sku uint64, txId *uint32) (*models.Product, error)
	GetBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error)
}

type CachedProductRepository struct {
	db      productDbRepository
	cache   *ProductCache        // nil when cache is disabled
	metrics CacheMetricsReporter // nil when metrics are disabled
}

func NewCachedProductRepository(
	db productDbRepository,
	productCache *ProductCache,
	metrics CacheMetricsReporter,
) *CachedProductRepository {
	return &CachedProductRepository{db: db, cache: productCache, metrics: metrics}
}

func (r *CachedProductRepository) GetBySku(ctx context.Context, sku uint64, txId *uint32) (*models.Product, error) {
	if r.cache != nil {
		productFromCache, err := r.cache.Get(ctx, sku)
		if err != nil {
			slog.WarnContext(ctx, "CachedProductRepository.GetBySku: cache get failed, falling back to DB",
				"sku", sku, "err", err)
		} else if productFromCache != nil {
			if txId == nil || productFromCache.TransactionId >= *txId {
				return productFromCache, nil
			}
			// Cache hit, but the cached xmin is older than required — treat as stale.
			if r.metrics != nil {
				r.metrics.ReportOperation("get", "stale", 0)
			}
			slog.DebugContext(ctx, "CachedProductRepository.GetBySku: stale xmin, fetching from DB",
				"sku", sku,
				"cached_xmin", productFromCache.TransactionId,
				"required_xmin", *txId,
			)
		}
	}

	product, err := r.db.GetBySku(ctx, sku, nil)
	if err != nil {
		return nil, err
	}

	if r.cache != nil && product != nil {
		go r.cache.Set(context.Background(), product)
	}

	return product, nil
}

func (r *CachedProductRepository) GetBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error) {
	if r.cache == nil {
		return r.db.GetBySkus(ctx, skus)
	}

	result := make([]*models.Product, 0, len(skus))
	var missedSkus []uint64

	for _, sku := range skus {
		//TODO: добавить батчевый запрос
		productFromCache, err := r.cache.Get(ctx, sku)
		if err != nil {
			slog.WarnContext(ctx, "CachedProductRepository.GetBySkus: cache get failed, treating as miss",
				"sku", sku, "err", err)
			missedSkus = append(missedSkus, sku)
			continue
		}
		if productFromCache == nil {
			missedSkus = append(missedSkus, sku)
			continue
		}
		result = append(result, productFromCache)
	}

	if len(missedSkus) > 0 {
		productsFromDb, err := r.db.GetBySkus(ctx, missedSkus)
		if err != nil {
			return nil, err
		}
		for _, productFromDb := range productsFromDb {
			//TODO: в одной горутине сетить весь батч, а не отдельные горутины под каждый продукт
			go r.cache.Set(context.Background(), productFromDb)
		}
		result = append(result, productsFromDb...)
	}

	return result, nil
}
