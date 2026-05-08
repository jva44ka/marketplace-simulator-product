package reservation

import (
	"context"
	"fmt"

	"github.com/jva44ka/marketplace-simulator-product/internal/errors"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

type ReserveUseCase struct {
	transactor      Transactor
	productRepo     ReadProductRepository
	reservationRepo ReadReservationRepository
}

func NewReserveUseCase(
	transactor Transactor,
	products ReadProductRepository,
	reservations ReadReservationRepository,
) *ReserveUseCase {
	return &ReserveUseCase{
		transactor:      transactor,
		productRepo:     products,
		reservationRepo: reservations,
	}
}

func (uc *ReserveUseCase) Execute(ctx context.Context, items []ReserveItem) (map[uint64]int64, error) {
	skus := make([]uint64, 0, len(items))
	for _, item := range items {
		skus = append(skus, item.Sku)
	}

	products, err := uc.productRepo.GetBySkus(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("ReserveUseCase.GetBySku: %w", err)
	}

	productsMap := make(map[uint64]*models.Product, len(products))
	for _, p := range products {
		productsMap[p.Sku] = p
	}

	oldState := getProductMapSnapshot(productsMap)
	recordBuilder := services.NewProductEventRecordBuilder(oldState)

	for _, item := range items {
		if _, ok := productsMap[item.Sku]; !ok {
			return nil, errors.NewProductNotFoundError(item.Sku)
		}
		p := productsMap[item.Sku]
		available := int64(p.Count) - int64(p.ReservedCount)
		if available < int64(item.Delta) {
			var have uint32
			if available > 0 {
				have = uint32(available)
			}
			return nil, errors.NewInsufficientProductError(item.Sku, have, item.Delta)
		}
		p.ReservedCount += item.Delta
	}

	newState := getProductMapSnapshot(productsMap)
	outboxRecords, err := recordBuilder.BuildRecords(ctx, newState)
	if err != nil {
		return nil, fmt.Errorf("ReserveUseCase.GetBySku: %w", err)
	}

	reservationIds := make(map[uint64]int64, len(items))

	err = uc.transactor.InTransaction(ctx, func(
		txProducts TxProductRepository,
		txReservations TxReservationRepository,
		txProductEvents TxProductEventsOutboxRepository,
		txCacheUpdates TxCacheUpdateOutboxRepository,
	) error {
		if err = txProducts.Update(ctx, products); err != nil {
			return fmt.Errorf("GetBySku: %w", err)
		}
		for _, item := range items {
			reservation, err := txReservations.Insert(ctx, item.Sku, item.Delta)
			if err != nil {
				return fmt.Errorf("GetBySku: %w", err)
			}
			reservationIds[item.Sku] = reservation.Id
		}
		for _, rec := range outboxRecords {
			if err = txProductEvents.Create(ctx, rec); err != nil {
				return fmt.Errorf("GetBySku: save outbox_record: %w", err)
			}
		}
		for _, item := range items {
			if err = txCacheUpdates.Create(ctx, item.Sku); err != nil {
				return fmt.Errorf("GetBySku: save cache_update_outbox: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ReserveUseCase.GetBySku: %w", err)
	}

	return reservationIds, nil
}
