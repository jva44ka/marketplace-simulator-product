package product

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/ozon-simulator-go-products/internal/errors"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
)

// TODO вынести в reservations service
func (s *Service) Reserve(ctx context.Context, products []UpdateCount) (map[uint64]int64, error) {
	existingProductsMap, err := validateProductsExist(ctx, products, s.db.Products())
	if err != nil {
		return nil, fmt.Errorf("ProductService.Reserve: %w", err)
	}

	for _, product := range products {
		existingProduct := existingProductsMap[product.Sku]
		if existingProduct.Count < product.Delta {
			return nil, errors.NewInsufficientProductError(product.Sku, existingProduct.Count, product.Delta)
		}
		existingProduct.Count -= product.Delta
	}

	reservationIds := make(map[uint64]int64, len(products))
	err = s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		if err = s.db.Products().WithTx(tx).UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
			return fmt.Errorf("ProductService.Reserve: %w", err)
		}

		var reservation models.Reservation
		reservationRepo := s.db.Reservations().WithTx(tx)

		for _, product := range products {
			reservation, err = reservationRepo.Insert(ctx, product.Sku, product.Delta)
			if err != nil {
				return fmt.Errorf("ProductService.Reserve: %w", err)
			}
			reservationIds[product.Sku] = reservation.Id
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ProductService.Reserve: %w", err)
	}

	return reservationIds, nil
}
