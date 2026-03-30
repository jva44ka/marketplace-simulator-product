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

	reservationIds := make(map[uint64]int64, len(reserveItems))

	err := s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		productsReadOnlyRepo := s.db.Products()
		reservationsReadOnlyRepo := s.db.Reservations()
		reservationsTxRepo := s.db.Reservations().WithTx(tx)

		products, err := productsReadOnlyRepo.GetProductsBySkus(ctx, skus)
		if err != nil {
			return fmt.Errorf("Reserve: %w", err)
		}

		productMap := make(map[uint64]*models.Product, len(products))
		for _, product := range products {
			productMap[product.Sku] = product
		}
		for _, reserveItem := range reserveItems {
			if _, ok := productMap[reserveItem.Sku]; !ok {
				return errors.NewProductNotFoundError(reserveItem.Sku)
			}
		}

		reservationSumsBySkus, err := reservationsReadOnlyRepo.GetSumBySkus(ctx, skus)
		if err != nil {
			return fmt.Errorf("Reserve: %w", err)
		}

		for _, reserveItem := range reserveItems {
			product := productMap[reserveItem.Sku]
			available := int64(product.Count) - int64(reservationSumsBySkus[reserveItem.Sku])
			if available < int64(reserveItem.Delta) {
				var have uint32
				if available > 0 {
					have = uint32(available)
				}
				return errors.NewInsufficientProductError(reserveItem.Sku, have, reserveItem.Delta)
			}
		}

		for _, item := range reserveItems {
			reservation, err := reservationsTxRepo.Insert(ctx, item.Sku, item.Delta)
			if err != nil {
				return fmt.Errorf("Reserve: %w", err)
			}
			//TODO: если передадут 2 одинаковых sku в запросе то тут будет перезапись
			//как вариант можно проверять sku в запрсое на уникальность
			reservationIds[item.Sku] = reservation.Id
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ReservationService.Reserve: %w", err)
	}

	return reservationIds, nil
}
