package product

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jva44ka/marketplace-simulator-product/internal/services/outbox"
)

func (s *Service) IncreaseCount(ctx context.Context, products []UpdateCount) error {
	existingProductsMap, err := validateProductsExist(ctx, products, s.products)
	if err != nil {
		return fmt.Errorf("ProductService.IncreaseCount: %w", err)
	}

	oldState := getProductMapSnapshot(existingProductsMap)
	recordBuilder := outbox.NewProductEventRecordBuilder(oldState)

	for _, product := range products {
		existingProductsMap[product.Sku].Count += product.Delta
	}

	newState := getProductMapSnapshot(existingProductsMap)
	outboxRecords, err := recordBuilder.BuildRecords(ctx, newState)
	if err != nil {
		return fmt.Errorf("ProductService.IncreaseCount: %w", err)
	}

	return s.transactor.InTransaction(ctx, func(
		txProducts TxProductRepository,
		txProductEvents TxProductEventsOutboxRepository,
		txCacheUpdates TxCacheUpdateOutboxRepository,
	) error {
		if err = txProducts.Update(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
			return fmt.Errorf("IncreaseCount: %w", err)
		}

		for _, outboxRecord := range outboxRecords {
			if err = txProductEvents.Create(ctx, outboxRecord); err != nil {
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
