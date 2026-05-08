package reservation

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

type ReleaseUseCase struct {
	transactor      Transactor
	productRepo     ReadProductRepository
	reservationRepo ReadReservationRepository
}

func NewReleaseUseCase(
	transactor Transactor,
	products ReadProductRepository,
	reservations ReadReservationRepository,
) *ReleaseUseCase {
	return &ReleaseUseCase{
		transactor:      transactor,
		productRepo:     products,
		reservationRepo: reservations,
	}
}

func (uc *ReleaseUseCase) Execute(ctx context.Context, ids []int64) error {
	reservations, err := uc.reservationRepo.GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("ReleaseUseCase.Execute: %w", err)
	}
	if len(reservations) == 0 {
		return nil
	}

	reservationSumsBySku := make(map[uint64]uint32, len(reservations))
	for _, r := range reservations {
		reservationSumsBySku[r.Sku] += r.Count
	}

	skus := slices.Collect(maps.Keys(reservationSumsBySku))
	products, err := uc.productRepo.GetBySkus(ctx, skus)
	if err != nil {
		return fmt.Errorf("ReleaseUseCase.Execute: %w", err)
	}

	productMap := make(map[uint64]*models.Product, len(products))
	for _, p := range products {
		productMap[p.Sku] = p
	}

	oldState := getProductMapSnapshot(productMap)
	recordBuilder := services.NewProductEventRecordBuilder(oldState)

	for _, p := range products {
		productMap[p.Sku].ReservedCount -= reservationSumsBySku[p.Sku]
	}

	newState := getProductMapSnapshot(productMap)
	outboxRecords, err := recordBuilder.BuildRecords(ctx, newState)
	if err != nil {
		return fmt.Errorf("ReleaseUseCase.Execute: %w", err)
	}

	return uc.transactor.InTransaction(ctx, func(
		txProducts TxProductRepository,
		txReservations TxReservationRepository,
		txProductEvents TxProductEventsOutboxRepository,
		txCacheUpdates TxCacheUpdateOutboxRepository,
	) error {
		if err = txProducts.Update(ctx, slices.Collect(maps.Values(productMap))); err != nil {
			return fmt.Errorf("Execute: %w", err)
		}
		if err = txReservations.DeleteByIds(ctx, ids); err != nil {
			return fmt.Errorf("Execute: %w", err)
		}
		for _, rec := range outboxRecords {
			if err = txProductEvents.Create(ctx, rec); err != nil {
				return fmt.Errorf("Execute: save outbox_record: %w", err)
			}
		}
		for sku := range reservationSumsBySku {
			if err = txCacheUpdates.Create(ctx, sku); err != nil {
				return fmt.Errorf("Execute: save cache_update_outbox: %w", err)
			}
		}
		return nil
	})
}
