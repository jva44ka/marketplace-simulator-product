package product

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/postgres"
)

type RepositoryMetrics interface {
	ReportRequest(method, status string)
	ReportOptimisticLockFailure()
}

type ProductRepository struct {
	pool    *pgxpool.Pool
	metrics RepositoryMetrics
}

func NewPgxRepository(pool *pgxpool.Pool, metrics RepositoryMetrics) *ProductRepository {
	return &ProductRepository{pool: pool, metrics: metrics}
}

type productRow struct {
	sku   int64
	price float64
	name  string
	count uint32
	xmin  uint32
}

// rowQuerier — минимальный интерфейс для SELECT-запросов.
type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (r *ProductRepository) querier(ctx context.Context) rowQuerier {
	if tx, ok := postgres.TxFromContext(ctx); ok {
		return tx
	}
	return r.pool
}

func (r *ProductRepository) GetProductBySku(ctx context.Context, sku uint64) (*Product, error) {
	products, err := r.GetProductsBySkus(ctx, []uint64{sku})
	if err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, nil
	}
	return products[0], nil
}

func (r *ProductRepository) GetProductsBySkus(ctx context.Context, skus []uint64) ([]*Product, error) {
	const query = `
SELECT sku, price, name, count, xmin
FROM products
WHERE sku = ANY($1);`

	rows, err := r.querier(ctx).Query(ctx, query, skus)
	if err != nil {
		r.metrics.ReportRequest("GetProductsBySkus", "error")
		return nil, fmt.Errorf("ProductRepository.GetProductsBySkus: %w", err)
	}
	defer rows.Close()

	products := make([]*Product, 0, len(skus))
	for rows.Next() {
		var row productRow
		if err = rows.Scan(&row.sku, &row.price, &row.name, &row.count, &row.xmin); err != nil {
			r.metrics.ReportRequest("GetProductsBySkus", "error")
			return nil, fmt.Errorf("ProductRepository.GetProductsBySkus: %w", err)
		}
		products = append(products, &Product{
			Sku:           uint64(row.sku),
			Price:         row.price,
			Name:          row.name,
			Count:         row.count,
			TransactionId: row.xmin,
		})
	}

	r.metrics.ReportRequest("GetProductsBySkus", "success")
	return products, nil
}

func (r *ProductRepository) UpdateCount(ctx context.Context, products []*Product) error {
	const query = `
UPDATE products
SET count = $3
WHERE sku = $1 AND xmin = $2;`

	return r.execBatch(ctx, "UpdateCount", products, query, func(p *Product) []any {
		return []any{int64(p.Sku), p.TransactionId, p.Count}
	})
}

func (r *ProductRepository) execBatch(
	ctx context.Context,
	method string,
	products []*Product,
	query string,
	args func(*Product) []any,
) error {
	do := func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, p := range products {
			batch.Queue(query, args(p)...)
		}

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		var affected int64
		for range products {
			tag, err := results.Exec()
			if err != nil {
				return fmt.Errorf("ProductRepository.%s: %w", method, err)
			}
			affected += tag.RowsAffected()
		}

		if affected != int64(len(products)) {
			r.metrics.ReportRequest(method, "error")
			r.metrics.ReportOptimisticLockFailure()
			return fmt.Errorf("ProductRepository.%s: optimistic lock failed, retry required", method)
		}

		r.metrics.ReportRequest(method, "success")
		return nil
	}

	if tx, ok := postgres.TxFromContext(ctx); ok {
		return do(tx)
	}
	return pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, do)
}
