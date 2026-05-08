package product

import (
	"context"

	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type ReadProductRepository interface {
	GetBySku(ctx context.Context, sku uint64, txId *uint32) (*models.Product, error)
	GetBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error)
}

type TxProductRepository interface {
	Update(ctx context.Context, products []*models.Product) error
}

type TxProductEventsOutboxRepository interface {
	Create(ctx context.Context, record models.ProductEventOutboxRecordNew) error
}

type TxCacheUpdateOutboxRepository interface {
	Create(ctx context.Context, sku uint64) error
}

type Transactor interface {
	InTransaction(ctx context.Context, fn func(
		products TxProductRepository,
		productEvents TxProductEventsOutboxRepository,
		cacheUpdates TxCacheUpdateOutboxRepository,
	) error) error
}
