package product

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

type IncreaseCountUseCase struct {
	transactor Transactor
	products   ReadProductRepository
}

func NewIncreaseCountUseCase(
	transactor Transactor,
	products ReadProductRepository,
) *IncreaseCountUseCase {
	return &IncreaseCountUseCase{
		transactor: transactor,
		products:   products,
	}
}

func (uc *IncreaseCountUseCase) IncreaseCount(ctx context.Context, products []UpdateCount) error {
	existingProductsMap, err := validateProductsExist(ctx, products, uc.products)
	if err != nil {
		return fmt.Errorf("IncreaseCount: %w", err)
	}

	oldState := getProductMapSnapshot(existingProductsMap)
	recordBuilder := services.NewProductEventRecordBuilder(oldState)

	for _, p := range products {
		existingProductsMap[p.Sku].Count += p.Delta
	}

	newState := getProductMapSnapshot(existingProductsMap)
	outboxRecords, err := recordBuilder.BuildRecords(ctx, newState)
	if err != nil {
		return fmt.Errorf("IncreaseCount: %w", err)
	}

	return uc.transactor.InTransaction(ctx, func(
		txProducts TxProductRepository,
		txProductEvents TxProductEventsOutboxRepository,
		txCacheUpdates TxCacheUpdateOutboxRepository,
	) error {
		if err = txProducts.Update(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
			return fmt.Errorf("IncreaseCount: %w", err)
		}
		for _, rec := range outboxRecords {
			if err = txProductEvents.Create(ctx, rec); err != nil {
				return fmt.Errorf("IncreaseCount: save outbox_record: %w", err)
			}
		}
		for _, p := range products {
			if err = txCacheUpdates.Create(ctx, p.Sku); err != nil {
				return fmt.Errorf("IncreaseCount: save cache_update_outbox: %w", err)
			}
		}
		return nil
	})
}
