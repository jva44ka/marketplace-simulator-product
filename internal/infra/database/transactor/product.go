package transactor

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/database/repository"
	ucProduct "github.com/jva44ka/marketplace-simulator-product/internal/usecases/product"
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
		products ucProduct.TxProductRepository,
		productEvents ucProduct.TxProductEventsOutboxRepository,
		cacheUpdates ucProduct.TxCacheUpdateOutboxRepository,
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
