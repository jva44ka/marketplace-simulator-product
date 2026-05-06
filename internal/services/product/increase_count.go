package product

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services/outbox"
)

func (s *Service) IncreaseCount(ctx context.Context, products []UpdateCount) error {
	existingProductsMap, err := validateProductsExist(ctx, products, s.db.ProductsRepo())
	if err != nil {
		return fmt.Errorf("ProductService.IncreaseCount: %w", err)
	}

	// Копируем old state до изменений
	//TODO: дубль в reservations/release и reservations/reserve
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

	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		if err = s.db.ProductsRepo().WithTx(tx).Update(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
			return fmt.Errorf("IncreaseCount: %w", err)
		}

		for _, outboxRecord := range outboxRecords {
			if err = s.db.ProductEventsOutboxRepo().WithTx(tx).Create(ctx, outboxRecord); err != nil {
				return fmt.Errorf("IncreaseCount: save outbox_record_builder: %w", err)
			}
		}

		cacheOutbox := s.db.CacheUpdateOutboxRepo().WithTx(tx)
		for _, p := range products {
			if err = cacheOutbox.Create(ctx, p.Sku); err != nil {
				return fmt.Errorf("IncreaseCount: save cache_update_outbox: %w", err)
			}
		}

		return nil
	})
}

func getProductMapSnapshot(productMap map[uint64]*models.Product) map[uint64]models.Product {
	snapshot := make(map[uint64]models.Product, len(productMap))

	for sku, productMapItem := range productMap {
		snapshot[sku] = *productMapItem
	}

	return snapshot
}
