package transactor

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/database/repository"
	serviceProduct "github.com/jva44ka/marketplace-simulator-product/internal/services/product"
)

type ProductServiceTransactor struct {
	pool           *pgxpool.Pool
	productMetrics repository.ProductRepositoryMetrics
}

func NewProductServiceTransactor(
	pool *pgxpool.Pool,
	productMetrics repository.ProductRepositoryMetrics,
) *ProductServiceTransactor {
	return &ProductServiceTransactor{
		pool:           pool,
		productMetrics: productMetrics,
	}
}

func (t *ProductServiceTransactor) InTransaction(
	ctx context.Context,
	fn func(
		products      serviceProduct.TxProductRepository,
		productEvents serviceProduct.TxProductEventsOutboxRepository,
		cacheUpdates  serviceProduct.TxCacheUpdateOutboxRepository,
	) error,
) error {
	return pgx.BeginTxFunc(ctx, t.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return fn(
			repository.NewProductPgxTxRepository(tx, t.productMetrics),
			repository.NewProductEventsOutboxTxRepository(tx),
			repository.NewCacheUpdateOutboxTxRepository(tx),
		)
	})
}
