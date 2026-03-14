package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/model"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/service"
)

type ProductRepository struct {
	pool *pgxpool.Pool
}

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

type ProductRow struct {
	sku   int64
	price float64
	name  string
	count uint32
}

type UpdateProductCount struct {
	Sku   uint64
	Delta uint32
}

func (r *ProductRepository) GetProductBySku(ctx context.Context, sku uint64) (*model.Product, error) {
	products, err := r.GetProductsBySkus(ctx, []uint64{sku})
	if err != nil {
		return nil, err
	}

	return products[0], nil
}

func (r *ProductRepository) GetProductsBySkus(ctx context.Context, skus []uint64) ([]*model.Product, error) {
	const query = `
SELECT sku, price, name, count
FROM products
WHERE sku = ANY($1);`

	rows, err := r.pool.Query(ctx, query, skus)
	if err != nil {
		return nil, fmt.Errorf("PgxRepository.GetProductsBySkus: %w", err)
	}
	defer rows.Close()

	products := make([]*model.Product, 0, len(skus))
	for rows.Next() {
		var row ProductRow
		if err := rows.Scan(&row.sku, &row.price, &row.name, &row.count); err != nil {
			return nil, fmt.Errorf("PgxRepository.GetProductsBySkus: %w", err)
		}
		products = append(products, &model.Product{
			Sku:   uint64(row.sku),
			Price: row.price,
			Name:  row.name,
			Count: row.count,
		})
	}

	return products, nil
}

func (r *ProductRepository) IncreaseCount(ctx context.Context, products []service.UpdateProductCount) error {
	const query = `
UPDATE products 
SET count = count + $2
WHERE sku = $1;`

	return pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, s := range products {
			batch.Queue(query, int64(s.Sku), s.Delta)
		}

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		var affected int64
		for range products {
			tag, err := results.Exec()
			if err != nil {
				return fmt.Errorf("PgxRepository.IncreaseCount: %w", err)
			}
			affected += tag.RowsAffected()
		}

		if affected != int64(len(products)) {
			//TODO: metric
			return fmt.Errorf("PgxRepository.IncreaseCount: some skus not found")
		}

		return nil
	})
}

func (r *ProductRepository) DecreaseCount(ctx context.Context, products []service.UpdateProductCount) error {
	const query = `
UPDATE products 
SET count = count - $2
WHERE sku = $1 AND count >= $2;`

	return pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, s := range products {
			batch.Queue(query, int64(s.Sku), s.Delta)
		}

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		var affected int64
		for range products {
			tag, err := results.Exec()
			if err != nil {
				return fmt.Errorf("PgxRepository.DecreaseCount: %w", err)
			}
			affected += tag.RowsAffected()
		}

		if affected != int64(len(products)) {
			//TODO: metric
			return fmt.Errorf("PgxRepository.DecreaseCount: insufficient stock or sku not found")
		}

		return nil
	})
}
