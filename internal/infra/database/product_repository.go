package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	appErrors "github.com/jva44ka/marketplace-simulator-product/internal/errors"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type ProductRepositoryMetrics interface {
	ReportRequest(method, status string, duration time.Duration)
	ReportOptimisticLockFailure()
}

type ProductPgxRepository struct {
	pool    *pgxpool.Pool
	metrics ProductRepositoryMetrics
}

func NewProductPgxRepository(pool *pgxpool.Pool, metrics ProductRepositoryMetrics) *ProductPgxRepository {
	return &ProductPgxRepository{pool: pool, metrics: metrics}
}

type productRow struct {
	sku           int64
	price         float64
	name          string
	count         uint32
	reservedCount uint32
	xmin          uint32
}

func (r *ProductPgxRepository) GetBySku(ctx context.Context, sku uint64, _ *uint32) (*models.Product, error) {
	products, err := r.GetBySkus(ctx, []uint64{sku})
	if err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, nil
	}
	return products[0], nil
}

func (r *ProductPgxRepository) GetBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error) {
	const query = `
SELECT sku, price, name, count, reserved_count, xmin
FROM products
WHERE sku = ANY($1);`

	start := time.Now()
	rows, err := r.pool.Query(ctx, query, skus)
	if err != nil {
		r.metrics.ReportRequest("GetBySkus", "error", time.Since(start))
		return nil, fmt.Errorf("ProductPgxRepository.GetBySkus: %w", err)
	}
	defer rows.Close()

	products := make([]*models.Product, 0, len(skus))
	for rows.Next() {
		var row productRow
		if err = rows.Scan(&row.sku, &row.price, &row.name, &row.count, &row.reservedCount, &row.xmin); err != nil {
			r.metrics.ReportRequest("GetBySkus", "error", time.Since(start))
			return nil, fmt.Errorf("ProductPgxRepository.GetBySkus: %w", err)
		}
		products = append(products, &models.Product{
			Sku:           uint64(row.sku),
			Price:         row.price,
			Name:          row.name,
			Count:         row.count,
			ReservedCount: row.reservedCount,
			TransactionId: row.xmin,
		})
	}

	r.metrics.ReportRequest("GetBySkus", "success", time.Since(start))
	return products, nil
}

func (r *ProductPgxRepository) WithTx(tx pgx.Tx) *ProductPgxTxRepository {
	return &ProductPgxTxRepository{tx: tx, metrics: r.metrics}
}

type ProductPgxTxRepository struct {
	tx      pgx.Tx
	metrics ProductRepositoryMetrics
}

func (r *ProductPgxTxRepository) Update(ctx context.Context, products []*models.Product) error {
	const query = `
UPDATE products
SET
    count = $3,
    reserved_count = $4
WHERE sku = $1 AND xmin = $2;`

	start := time.Now()
	batch := &pgx.Batch{}
	for _, p := range products {
		batch.Queue(query, int64(p.Sku), p.TransactionId, p.Count, p.ReservedCount)
	}

	results := r.tx.SendBatch(ctx, batch)
	defer results.Close()

	var affected int64
	for range products {
		tag, err := results.Exec()
		if err != nil {
			return fmt.Errorf("ProductPgxTxRepository.Update: %w", err)
		}
		affected += tag.RowsAffected()
	}

	if affected != int64(len(products)) {
		r.metrics.ReportRequest("Update", "error", time.Since(start))
		r.metrics.ReportOptimisticLockFailure()
		return fmt.Errorf("ProductPgxTxRepository.Update: %w", appErrors.NewOptimisticLockError())
	}

	r.metrics.ReportRequest("Update", "success", time.Since(start))
	return nil
}
