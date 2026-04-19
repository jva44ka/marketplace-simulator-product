package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
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

type ProductPgxTxRepository struct {
	tx      pgx.Tx
	metrics RepositoryMetrics
}

func (r *ProductPgxRepository) WithTx(tx pgx.Tx) services.ProductTxRepository {
	return &ProductPgxTxRepository{tx: tx, metrics: r.metrics}
}

type productRow struct {
	sku           int64
	price         float64
	name          string
	count         uint32
	reservedCount uint32
	xmin          uint32
}

func (r *ProductPgxRepository) GetBySku(ctx context.Context, sku uint64) (*models.Product, error) {
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

	rows, err := r.pool.Query(ctx, query, skus)
	if err != nil {
		r.metrics.ReportRequest("GetBySkus", "error")
		return nil, fmt.Errorf("ProductRepository.GetBySkus: %w", err)
	}
	defer rows.Close()

	products := make([]*models.Product, 0, len(skus))
	for rows.Next() {
		var row productRow
		if err = rows.Scan(&row.sku, &row.price, &row.name, &row.count, &row.reservedCount, &row.xmin); err != nil {
			r.metrics.ReportRequest("GetBySkus", "error")
			return nil, fmt.Errorf("ProductRepository.GetBySkus: %w", err)
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

	r.metrics.ReportRequest("GetBySkus", "success")
	return products, nil
}

func (r *ProductPgxTxRepository) Update(ctx context.Context, products []*models.Product) error {
	const query = `
UPDATE products
SET 
    count = $3, 
    reserved_count = $4
WHERE sku = $1 AND xmin = $2;`

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
			return fmt.Errorf("ProductRepository.Update: %w", err)
		}
		affected += tag.RowsAffected()
	}

	if affected != int64(len(products)) {
		r.metrics.ReportRequest("Update", "error")
		r.metrics.ReportOptimisticLockFailure()
		return fmt.Errorf("ProductRepository.Update: optimistic lock failed, retry required")
	}

	r.metrics.ReportRequest("Update", "success")
	return nil
}
