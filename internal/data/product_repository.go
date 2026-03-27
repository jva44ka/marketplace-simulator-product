package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/models"
)

type RepositoryMetrics interface {
	ReportRequest(method, status string)
	ReportOptimisticLockFailure()
}

type ProductPgxRepository struct {
	pool    *pgxpool.Pool
	tx      pgx.Tx
	metrics RepositoryMetrics
}

func NewProductPgxRepository(pool *pgxpool.Pool, metrics RepositoryMetrics) *ProductPgxRepository {
	return &ProductPgxRepository{pool: pool, metrics: metrics}
}

func NewProductTxPgxRepository(tx pgx.Tx, metrics RepositoryMetrics) *ProductPgxRepository {
	return &ProductPgxRepository{tx: tx, metrics: metrics}
}

type productRow struct {
	sku   int64
	price float64
	name  string
	count uint32
	xmin  uint32
}

type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (r *ProductPgxRepository) conn() rowQuerier {
	if r.tx != nil {
		return r.tx
	}
	return r.pool
}

func (r *ProductPgxRepository) GetProductBySku(ctx context.Context, sku uint64) (*models.Product, error) {
	products, err := r.GetProductsBySkus(ctx, []uint64{sku})
	if err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, nil
	}
	return products[0], nil
}

func (r *ProductPgxRepository) GetProductsBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error) {
	const query = `
SELECT sku, price, name, count, xmin
FROM products
WHERE sku = ANY($1);`

	rows, err := r.conn().Query(ctx, query, skus)
	if err != nil {
		r.metrics.ReportRequest("GetProductsBySkus", "error")
		return nil, fmt.Errorf("ProductRepository.GetProductsBySkus: %w", err)
	}
	defer rows.Close()

	products := make([]*models.Product, 0, len(skus))
	for rows.Next() {
		var row productRow
		if err = rows.Scan(&row.sku, &row.price, &row.name, &row.count, &row.xmin); err != nil {
			r.metrics.ReportRequest("GetProductsBySkus", "error")
			return nil, fmt.Errorf("ProductRepository.GetProductsBySkus: %w", err)
		}
		products = append(products, &models.Product{
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

func (r *ProductPgxRepository) UpdateCount(ctx context.Context, products []*models.Product) error {
	const query = `
UPDATE products
SET count = $3
WHERE sku = $1 AND xmin = $2;`

	return r.execBatch(ctx, "UpdateCount", products, query, func(p *models.Product) []any {
		return []any{int64(p.Sku), p.TransactionId, p.Count}
	})
}

func (r *ProductPgxRepository) execBatch(
	ctx context.Context,
	method string,
	products []*models.Product,
	query string,
	args func(*models.Product) []any,
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

	if r.tx != nil {
		return do(r.tx)
	}
	return pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, do)
}
