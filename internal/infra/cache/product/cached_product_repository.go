package cache

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

var _ services.ProductRepository = (*CachedProductRepository)(nil)

// CachedProductRepository decorates services.ProductRepository with a Redis
// read-through cache.
//
// GetBySku supports a read-your-own-writes guarantee via the optional txId
// parameter: the cached entry is only served when its stored xmin is at least
// as large as the caller's known xmin.
//
// GetBySkus checks the cache per-SKU and batches the misses into a single DB
// query. Stale xmin values from the cache are safe here: the subsequent UPDATE
// uses optimistic locking (WHERE xmin = $cached_xmin), so a stale entry
// produces an OptimisticLockError — the same outcome as a concurrent
// modification — which the client is expected to retry.
type CachedProductRepository struct {
	db           services.ProductRepository
	productCache *ProductCache // nil when cache is disabled
}

// NewCachedProductRepository wraps db with a Redis cache layer.
// Pass nil for productCache for transparent pass-through behaviour.
func NewCachedProductRepository(inner services.ProductRepository, productCache *ProductCache) *CachedProductRepository {
	return &CachedProductRepository{db: inner, productCache: productCache}
}

// GetBySku tries the cache first, then falls back to the DB.
// txId is the caller's optional PostgreSQL xmin from a prior mutation response;
// pass nil when no freshness guarantee is needed.
func (r *CachedProductRepository) GetBySku(ctx context.Context, sku uint64, txId *uint32) (*models.Product, error) {
	if r.productCache != nil {
		productFromCache, err := r.productCache.Get(ctx, sku)
		if err != nil {
			slog.WarnContext(ctx, "CachedProductRepository.GetBySku: cache get failed, falling back to DB",
				"sku", sku, "err", err)
		} else if productFromCache != nil {
			if txId == nil || productFromCache.TransactionId >= *txId {
				return productFromCache, nil
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
	if r.productCache != nil && product != nil {
		go r.productCache.Set(context.Background(), product)
	}
	return product, nil
}

func (r *CachedProductRepository) GetBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error) {
	if r.productCache == nil {
		return r.db.GetBySkus(ctx, skus)
	}

	result := make([]*models.Product, 0, len(skus))
	var missedSkus []uint64

	for _, sku := range skus {
		cached, err := r.productCache.Get(ctx, sku)
		if err != nil {
			slog.WarnContext(ctx, "CachedProductRepository.GetBySkus: cache get failed, treating as miss",
				"sku", sku, "err", err)
			missedSkus = append(missedSkus, sku)
			continue
		}
		if cached == nil {
			missedSkus = append(missedSkus, sku)
			continue
		}
		result = append(result, cached)
	}

	if len(missedSkus) > 0 {
		fromDB, err := r.db.GetBySkus(ctx, missedSkus)
		if err != nil {
			return nil, err
		}
		for _, p := range fromDB {
			go r.productCache.Set(context.Background(), p)
		}
		result = append(result, fromDB...)
	}

	return result, nil
}

func (r *CachedProductRepository) WithTx(tx pgx.Tx) services.ProductTxRepository {
	return r.db.WithTx(tx)
}
