package transactor

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/database/repository"
	serviceProduct "github.com/jva44ka/marketplace-simulator-product/internal/services/product"
)

type ProductServiceTransactor struct {
	pool                *pgxpool.Pool
	products            *repository.ProductPgxRepository
	productEventsOutbox *repository.ProductEventsOutboxRepository
	cacheUpdateOutbox   *repository.CacheUpdateOutboxRepository
}

func NewProductServiceTransactor(
	pool *pgxpool.Pool,
	products *repository.ProductPgxRepository,
	productEventsOutbox *repository.ProductEventsOutboxRepository,
	cacheUpdateOutbox *repository.CacheUpdateOutboxRepository,
) *ProductServiceTransactor {
	return &ProductServiceTransactor{
		pool:                pool,
		products:            products,
		productEventsOutbox: productEventsOutbox,
		cacheUpdateOutbox:   cacheUpdateOutbox,
	}
}

func (t *ProductServiceTransactor) InTransaction(
	ctx context.Context,
	fn func(
	products serviceProduct.TxProductRepository,
	productEvents serviceProduct.TxProductEventsOutboxRepository,
	cacheUpdates serviceProduct.TxCacheUpdateOutboxRepository,
) error,
) error {
	return pgx.BeginTxFunc(ctx, t.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return fn(
			t.products.WithTx(tx),
			t.productEventsOutbox.WithTx(tx),
			t.cacheUpdateOutbox.WithTx(tx),
		)
	})
}
