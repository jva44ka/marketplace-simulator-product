package reservation

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
	"github.com/jva44ka/ozon-simulator-go-products/internal/services/outbox"
)

func (s *Service) Confirm(ctx context.Context, ids []int64) error {
	reservations, err := s.db.ReservationsRepo().GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("ReservationService.Confirm: %w", err)
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

	// Копируем old state до изменений
	//TODO: дубль в products/increase
	oldState := getProductMapSnapshot(productMap)
	recordBuilder := outbox.NewProductEventsRecordBuilder(oldState)

	for _, product := range products {
		delta := reservationSumsBySku[product.Sku]
		productMap[product.Sku].Count -= delta
		productMap[product.Sku].ReservedCount -= delta
	}

	newState := getProductMapSnapshot(productMap)
	outboxRecords, err := recordBuilder.BuildRecords(newState)
	if err != nil {
		return fmt.Errorf("ProductService.Confirm: %w", err)
	}

	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		if err = s.db.ProductsRepo().WithTx(tx).Update(ctx, slices.Collect(maps.Values(productMap))); err != nil {
			return fmt.Errorf("Confirm: %w", err)
		}

		if err = s.db.ReservationsRepo().WithTx(tx).DeleteByIds(ctx, ids); err != nil {
			return fmt.Errorf("Confirm: %w", err)
		}

		//TODO: сделать батчевую вставку
		for _, outboxRecord := range outboxRecords {
			if err = s.db.ProductEventsOutboxRepo().WithTx(tx).Create(ctx, outboxRecord); err != nil {
				return fmt.Errorf("Confirm: save outbox_record_builder: %w", err)
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
