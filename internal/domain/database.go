package data

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/product"
)

type Transactor struct {
	pool               *pgxpool.Pool
	productMetrics     RepositoryMetrics
	reservationMetrics Metrics
}
