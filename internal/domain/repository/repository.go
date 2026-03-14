package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/model"
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
	const query = `
SELECT 
    sku, 
    price, 
    name, 
    count, 
    xmin AS TransactionId
FROM 
    products 
WHERE 
    sku = $1;`

	row := r.pool.QueryRow(ctx, query, int64(sku))

	var productRow = ProductRow{}

	err := row.Scan(&productRow.sku, &productRow.price, &productRow.name, &productRow.count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrProductNotFound
		}
		return nil, fmt.Errorf("ProductRepository.GetProductBySku: %w", err)
	}

	return &model.Product{
		Sku:   uint64(productRow.sku),
		Price: productRow.price,
		Name:  productRow.name,
		Count: productRow.count,
	}, nil
}

func (r *ProductRepository) IncreaseCount(ctx context.Context, products []UpdateProductCount) error {
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

func (r *ProductRepository) DecreaseCount(ctx context.Context, products []UpdateProductCount) error {
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
