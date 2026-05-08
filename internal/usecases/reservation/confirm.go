package reservation

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type ConfirmUseCase struct {
	transactor      Transactor
	productRepo     ReadProductRepository
	reservationRepo ReadReservationRepository
}

func NewConfirmUseCase(
	transactor Transactor,
	products ReadProductRepository,
	reservations ReadReservationRepository,
) *ConfirmUseCase {
	return &ConfirmUseCase{
		transactor:      transactor,
		productRepo:     products,
		reservationRepo: reservations,
	}
}

func (uc *ConfirmUseCase) Execute(ctx context.Context, ids []int64) error {
	reservations, err := uc.reservationRepo.GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("ConfirmUseCase.Execute: %w", err)
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
		return fmt.Errorf("ConfirmUseCase.Execute: %w", err)
	}

	productMap := make(map[uint64]*models.Product, len(products))
	for _, p := range products {
		productMap[p.Sku] = p
	}

	for _, p := range products {
		delta := reservationSumsBySku[p.Sku]
		productMap[p.Sku].Count -= delta
		productMap[p.Sku].ReservedCount -= delta
	}

	return uc.transactor.InTransaction(ctx, func(
		txProducts TxProductRepository,
		txReservations TxReservationRepository,
		_ TxProductEventsOutboxRepository,
		txCacheUpdates TxCacheUpdateOutboxRepository,
	) error {
		if err = txProducts.Update(ctx, slices.Collect(maps.Values(productMap))); err != nil {
			return fmt.Errorf("Execute: %w", err)
		}
		if err = txReservations.DeleteByIds(ctx, ids); err != nil {
			return fmt.Errorf("Execute: %w", err)
		}
		for sku := range reservationSumsBySku {
			if err = txCacheUpdates.Create(ctx, sku); err != nil {
				return fmt.Errorf("Execute: save cache_update_outbox: %w", err)
			}
		}
		return nil
	})
}
