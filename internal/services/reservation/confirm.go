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
	reservations, err := s.reservations.GetByIds(ctx, ids)
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
	products, err := s.products.GetBySkus(ctx, skus)
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

	return s.transactor.InTransaction(ctx, func(tx pgx.Tx) error {
		if err = s.products.WithTx(tx).Update(ctx, slices.Collect(maps.Values(productMap))); err != nil {
			return fmt.Errorf("Confirm: %w", err)
		}

		if err = s.reservations.WithTx(tx).DeleteByIds(ctx, ids); err != nil {
			return fmt.Errorf("Confirm: %w", err)
		}

		//TODO: сделать батчевую вставку
		for sku := range reservationSumsBySku {
			if err = s.cacheOutbox.WithTx(tx).Create(ctx, sku); err != nil {
				return fmt.Errorf("Confirm: save cache_update_outbox: %w", err)
			}
		}

		return nil
	})
}
