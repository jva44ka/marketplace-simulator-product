package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Transactor begins DB transactions and hands the live pgx.Tx to the caller.
// Services call repo.WithTx(tx) inside fn to bind repositories to the transaction.
type Transactor struct {
	pool *pgxpool.Pool
}

func NewTransactor(pool *pgxpool.Pool) *Transactor {
	return &Transactor{pool: pool}
}

func (t *Transactor) InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, t.pool, pgx.TxOptions{}, fn)
}
