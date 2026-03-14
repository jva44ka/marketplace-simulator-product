package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/model"
)

type PgxRepository struct {
	pool *pgxpool.Pool
}

func NewPgxRepository(pool *pgxpool.Pool) *PgxRepository {
	return &PgxRepository{pool: pool}
}

type ProductRow struct {
	sku   int64
	price float64
	name  string
	count uint32
}

func (r *PgxRepository) GetProductBySku(ctx context.Context, sku uint64) (*model.Product, error) {
	const query = `
SELECT sku, price, name, count 
FROM products 
WHERE sku = $1;`

	row := r.pool.QueryRow(ctx, query, int64(sku))

	var productRow = ProductRow{}

	err := row.Scan(&productRow.sku, &productRow.price, &productRow.name, &productRow.count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrProductNotFound
		}
		return nil, fmt.Errorf("PgxRepository.GetProductBySku: %w", err)
	}

	// Преобразуем типы в модель приложения
	result := &model.Product{
		Sku:   uint64(productRow.sku),
		Price: productRow.price,
		Name:  productRow.name,
		Count: productRow.count,
	}

	return result, nil
}
