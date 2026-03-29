package product

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// TODO вынести в reservations service
func (s *Service) ConfirmReservations(ctx context.Context, ids []int64) error {
	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		return s.db.Reservations().WithTx(tx).DeleteByIds(ctx, ids)
	})
}
