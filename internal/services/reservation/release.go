package reservation

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
)

func (s *Service) Release(ctx context.Context, ids []int64) error {
	reservations, err := s.db.ReservationsRepo().GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("ReservationService.Release: %w", err)
	}

	reservationSumsBySku := make(map[uint64]uint32, len(reservations))
	for _, reservation := range reservations {
		reservationSumsBySku[reservation.Sku] += reservation.Count
	}

	skus := slices.Collect(maps.Keys(reservationSumsBySku))
	products, err := s.db.ProductsRepo().GetBySkus(ctx, skus)
	if err != nil {
		return fmt.Errorf("ReservationService.Release: %w", err)
	}

	productMap := make(map[uint64]*models.Product, len(products))
	for _, product := range products {
		productMap[product.Sku] = product
	}

	for _, product := range products {
		productMap[product.Sku].ReservedCount -= reservationSumsBySku[product.Sku]
	}

	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		if err = s.db.ProductsRepo().WithTx(tx).Update(ctx, slices.Collect(maps.Values(productMap))); err != nil {
			return fmt.Errorf("Release: %w", err)
		}
		return s.db.ReservationsRepo().WithTx(tx).DeleteByIds(ctx, ids)
	})
}
