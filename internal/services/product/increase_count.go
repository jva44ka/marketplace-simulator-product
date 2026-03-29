package product

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
)

func (s *Service) IncreaseCount(ctx context.Context, products []UpdateCount) error {
	existingProductsMap, err := validateProductsExist(ctx, products, s.db.Products())
	if err != nil {
		return fmt.Errorf("ProductService.IncreaseCount: %w", err)
	}
	for _, p := range products {
		existingProductsMap[p.Sku].Count += p.Delta
	}
	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		return s.db.Products().WithTx(tx).UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap)))
	})
}
