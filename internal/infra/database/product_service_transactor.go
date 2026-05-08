package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	svcProduct "github.com/jva44ka/marketplace-simulator-product/internal/services/product"
)

type ProductServiceTransactor struct {
	pool                *pgxpool.Pool
	products            *ProductPgxRepository
	productEventsOutbox *ProductEventsOutboxRepository
	cacheUpdateOutbox   *CacheUpdateOutboxRepository
}

func NewProductServiceTransactor(
	pool *pgxpool.Pool,
	products *ProductPgxRepository,
	productEventsOutbox *ProductEventsOutboxRepository,
	cacheUpdateOutbox *CacheUpdateOutboxRepository,
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
		products svcProduct.TxProductRepository,
		productEvents svcProduct.TxProductEventsOutboxRepository,
		cacheUpdates svcProduct.TxCacheUpdateOutboxRepository,
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
