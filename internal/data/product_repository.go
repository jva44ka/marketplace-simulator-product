package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/models"
)

type RepositoryMetrics interface {
	ReportRequest(method, status string)
	ReportOptimisticLockFailure()
}

type ProductPgxRepository struct {
	pool    *pgxpool.Pool
	metrics RepositoryMetrics
}

func NewProductPgxRepository(pool *pgxpool.Pool, metrics RepositoryMetrics) *ProductPgxRepository {
	return &ProductPgxRepository{pool: pool, metrics: metrics}
}

func (r *ProductPgxRepository) WithTx(tx pgx.Tx) domain.ProductWriteRepository {
	return &ProductPgxTxRepository{tx: tx, metrics: r.metrics}
}

type ProductPgxTxRepository struct {
	tx      pgx.Tx
	metrics RepositoryMetrics
}

type productRow struct {
	sku   int64
	price float64
	name  string
	count uint32
	xmin  uint32
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

	rows, err := r.pool.Query(ctx, query, skus)
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

func (r *ProductPgxTxRepository) UpdateCount(ctx context.Context, products []*models.Product) error {
	const query = `
UPDATE products
SET count = $3
WHERE sku = $1 AND xmin = $2;`

	batch := &pgx.Batch{}
	for _, p := range products {
		batch.Queue(query, int64(p.Sku), p.TransactionId, p.Count)
	}

	results := r.tx.SendBatch(ctx, batch)
	defer results.Close()

	var affected int64
	for range products {
		tag, err := results.Exec()
		if err != nil {
			return fmt.Errorf("ProductRepository.UpdateCount: %w", err)
		}
		affected += tag.RowsAffected()
	}

	if affected != int64(len(products)) {
		r.metrics.ReportRequest("UpdateCount", "error")
		r.metrics.ReportOptimisticLockFailure()
		return fmt.Errorf("ProductRepository.UpdateCount: optimistic lock failed, retry required")
	}

	r.metrics.ReportRequest("UpdateCount", "success")
	return nil
}
