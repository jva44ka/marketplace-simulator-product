package product

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
)

func (s *Service) ReleaseReservations(ctx context.Context, ids []int64) error {
	reservations, err := s.db.Reservations().GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("ProductService.ReleaseReservations: %w", err)
	}

	products := make([]UpdateCount, len(reservations))
	for i, r := range reservations {
		products[i] = UpdateCount{Sku: r.Sku, Delta: r.Count}
	}

	existingProductsMap, err := validateProductsExist(ctx, products, s.db.Products())
	if err != nil {
		return fmt.Errorf("ProductService.ReleaseReservations: %w", err)
	}

	for _, product := range products {
		existingProductsMap[product.Sku].Count += product.Delta
	}

	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		if err = s.db.Products().WithTx(tx).UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
			return fmt.Errorf("ProductService.ReleaseReservations: %w", err)
		}
		return s.db.Reservations().WithTx(tx).DeleteByIds(ctx, ids)
	})
}

func (s *Service) ConfirmReservations(ctx context.Context, ids []int64) error {
	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		return s.db.Reservations().WithTx(tx).DeleteByIds(ctx, ids)
	})
}
