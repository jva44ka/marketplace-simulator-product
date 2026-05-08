package reservation

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services/outbox"
)

func (s *Service) Release(ctx context.Context, ids []int64) error {
	reservations, err := s.reservationRepo.GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("ReservationService.Release: %w", err)
	}

	if len(reservations) == 0 {
		return nil // already processed, idempotent no-op
	}

	reservationSumsBySku := make(map[uint64]uint32, len(reservations))
	for _, reservation := range reservations {
		reservationSumsBySku[reservation.Sku] += reservation.Count
	}

	skus := slices.Collect(maps.Keys(reservationSumsBySku))
	products, err := s.productRepo.GetBySkus(ctx, skus)
	if err != nil {
		return fmt.Errorf("ReservationService.Release: %w", err)
	}

	productMap := make(map[uint64]*models.Product, len(products))
	for _, product := range products {
		productMap[product.Sku] = product
	}

	oldState := getProductMapSnapshot(productMap)
	recordBuilder := outbox.NewProductEventRecordBuilder(oldState)

	for _, product := range products {
		productMap[product.Sku].ReservedCount -= reservationSumsBySku[product.Sku]
	}

	newState := getProductMapSnapshot(productMap)
	outboxRecords, err := recordBuilder.BuildRecords(ctx, newState)
	if err != nil {
		return fmt.Errorf("ReservationService.Release: %w", err)
	}

	return s.transactor.InTransaction(ctx, func(
		txProducts TxProductRepository,
		txReservations TxReservationRepository,
		txProductEvents TxProductEventsOutboxRepository,
		txCacheUpdates TxCacheUpdateOutboxRepository,
	) error {
		if err = txProducts.Update(ctx, slices.Collect(maps.Values(productMap))); err != nil {
			return fmt.Errorf("Release: %w", err)
		}

		if err = txReservations.DeleteByIds(ctx, ids); err != nil {
			return fmt.Errorf("Release: %w", err)
		}

		//TODO: сделать батчевую вставку
		for _, outboxRecord := range outboxRecords {
			if err = txProductEvents.Create(ctx, outboxRecord); err != nil {
				return fmt.Errorf("Release: save outbox_record: %w", err)
			}
		}

		//TODO: сделать батчевую вставку
		for sku := range reservationSumsBySku {
			if err = txCacheUpdates.Create(ctx, sku); err != nil {
				return fmt.Errorf("Release: save cache_update_outbox: %w", err)
			}
		}

		return nil
	})
}
