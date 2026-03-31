package reservation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/ozon-simulator-go-products/internal/errors"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
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

	products, err := s.db.Products().GetBySkus(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("ReservationService.Reserve: %w", err)
	}

	productsMap := make(map[uint64]*models.Product, len(products))
	for _, product := range products {
		productsMap[product.Sku] = product
	}

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

	reservationIds := make(map[uint64]int64, len(reserveItems))

	err = s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		productsTxRepo := s.db.Products().WithTx(tx)
		reservationsTxRepo := s.db.Reservations().WithTx(tx)

		if err = productsTxRepo.Update(ctx, products); err != nil {
			return fmt.Errorf("Reserve: %w", err)
		}

		for _, item := range reserveItems {
			reservation, err := reservationsTxRepo.Insert(ctx, item.Sku, item.Delta)
			if err != nil {
				return fmt.Errorf("Reserve: %w", err)
			}
			reservationIds[item.Sku] = reservation.Id
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ReservationService.Reserve: %w", err)
	}

	return reservationIds, nil
}
