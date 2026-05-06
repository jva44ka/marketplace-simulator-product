package reservation

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

func (s *Service) Confirm(ctx context.Context, ids []int64) error {
	reservations, err := s.db.ReservationsRepo().GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("ReservationService.Confirm: %w", err)
	}

	if len(reservations) == 0 {
		return nil // already processed, idempotent no-op
	}

	reservationSumsBySku := make(map[uint64]uint32, len(reservations))
	for _, reservation := range reservations {
		reservationSumsBySku[reservation.Sku] += reservation.Count
	}

	skus := slices.Collect(maps.Keys(reservationSumsBySku))
	products, err := s.db.ProductsRepo().GetBySkus(ctx, skus)
	if err != nil {
		return fmt.Errorf("ReservationService.Confirm: %w", err)
	}

	productMap := make(map[uint64]*models.Product, len(products))
	for _, product := range products {
		productMap[product.Sku] = product
	}

	for _, product := range products {
		delta := reservationSumsBySku[product.Sku]
		productMap[product.Sku].Count -= delta
		productMap[product.Sku].ReservedCount -= delta
	}

	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		if err = s.db.ProductsRepo().WithTx(tx).Update(ctx, slices.Collect(maps.Values(productMap))); err != nil {
			return fmt.Errorf("Confirm: %w", err)
		}

		if err = s.db.ReservationsRepo().WithTx(tx).DeleteByIds(ctx, ids); err != nil {
			return fmt.Errorf("Confirm: %w", err)
		}

		cacheOutbox := s.db.CacheUpdateOutboxRepo().WithTx(tx)
		for sku := range reservationSumsBySku {
			if err = cacheOutbox.Create(ctx, sku); err != nil {
				return fmt.Errorf("Confirm: save cache_update_outbox: %w", err)
			}
		}

		return nil
	})
}
