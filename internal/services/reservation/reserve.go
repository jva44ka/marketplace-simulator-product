package reservation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/marketplace-simulator-product/internal/errors"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services/outbox"
)

type ReserveItem struct {
	Sku   uint64
	Delta uint32
}

func (s *Service) Reserve(ctx context.Context, reserveItems []ReserveItem) (map[uint64]int64, error) {
	skus := make([]uint64, 0, len(reserveItems))
	for _, reserveItem := range reserveItems {
		skus = append(skus, reserveItem.Sku)
	}

	products, err := s.db.ProductsRepo().GetBySkus(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("ReservationService.Reserve: %w", err)
	}

	productsMap := make(map[uint64]*models.Product, len(products))
	for _, product := range products {
		productsMap[product.Sku] = product
	}

	oldState := getProductMapSnapshot(productsMap)
	recordBuilder := outbox.NewProductEventRecordBuilder(oldState)

	for _, reserveItem := range reserveItems {
		//Проверяем наличие продукта
		if _, ok := productsMap[reserveItem.Sku]; !ok {
			return nil, errors.NewProductNotFoundError(reserveItem.Sku)
		}

		//Проверяем достаточно ли продукта
		product := productsMap[reserveItem.Sku]
		available := int64(product.Count) - int64(product.ReservedCount)
		if available < int64(reserveItem.Delta) {
			var have uint32
			if available > 0 {
				have = uint32(available)
			}
			return nil, errors.NewInsufficientProductError(reserveItem.Sku, have, reserveItem.Delta)
		}

		//Резервируем
		product.ReservedCount += reserveItem.Delta
	}

	newState := getProductMapSnapshot(productsMap)
	outboxRecords, err := recordBuilder.BuildRecords(ctx, newState)
	if err != nil {
		return nil, fmt.Errorf("ReservationService.Reserve: %w", err)
	}

	reservationIds := make(map[uint64]int64, len(reserveItems))

	err = s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		productsTxRepo := s.db.ProductsRepo().WithTx(tx)
		reservationsTxRepo := s.db.ReservationsRepo().WithTx(tx)

		if err = productsTxRepo.Update(ctx, products); err != nil {
			return fmt.Errorf("Reserve: %w", err)
		}

		//TODO: сделать батчевую вставку
		for _, item := range reserveItems {
			reservation, err := reservationsTxRepo.Insert(ctx, item.Sku, item.Delta)
			if err != nil {
				return fmt.Errorf("Reserve: %w", err)
			}
			reservationIds[item.Sku] = reservation.Id
		}

		//TODO: сделать батчевую вставку
		for _, outboxRecord := range outboxRecords {
			if err = s.db.ProductEventsOutboxRepo().WithTx(tx).Create(ctx, outboxRecord); err != nil {
				return fmt.Errorf("Reserve: save outbox_record: %w", err)
			}
		}

		cacheOutbox := s.db.CacheUpdateOutboxRepo().WithTx(tx)
		for _, item := range reserveItems {
			if err = cacheOutbox.Create(ctx, item.Sku); err != nil {
				return fmt.Errorf("Reserve: save cache_update_outbox: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ReservationService.Reserve: %w", err)
	}

	return reservationIds, nil
}
