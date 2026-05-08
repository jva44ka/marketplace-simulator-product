package reservation

import (
	"context"
	"time"

	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type ReadProductRepository interface {
	GetBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error)
}

type ReadReservationRepository interface {
	GetByIds(ctx context.Context, ids []int64) ([]models.Reservation, error)
	GetExpired(ctx context.Context, cutoff time.Time) ([]models.Reservation, error)
}

type TxProductRepository interface {
	Update(ctx context.Context, products []*models.Product) error
}

type TxReservationRepository interface {
	Insert(ctx context.Context, sku uint64, count uint32) (models.Reservation, error)
	DeleteByIds(ctx context.Context, ids []int64) error
}

type TxProductEventsOutboxRepository interface {
	Create(ctx context.Context, record models.ProductEventOutboxRecordNew) error
}

type TxCacheUpdateOutboxRepository interface {
	Create(ctx context.Context, sku uint64) error
}

type ReserveItem struct {
	Sku   uint64
	Delta uint32
}

type Transactor interface {
	InTransaction(ctx context.Context, fn func(
		products TxProductRepository,
		reservations TxReservationRepository,
		productEvents TxProductEventsOutboxRepository,
		cacheUpdates TxCacheUpdateOutboxRepository,
	) error) error
}
