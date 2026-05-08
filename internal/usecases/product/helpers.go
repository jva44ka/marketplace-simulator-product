package product

import (
	"context"
	"fmt"

	"github.com/jva44ka/marketplace-simulator-product/internal/errors"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

func validateProductsExist(
	ctx context.Context,
	products []UpdateCount,
	repo ReadProductRepository,
) (map[uint64]*models.Product, error) {
	skus := make([]uint64, 0, len(products))
	for _, p := range products {
		skus = append(skus, p.Sku)
	}

	existingProducts, err := repo.GetBySkus(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("validateProductsExist: %w", err)
	}

	existingProductsMap := make(map[uint64]*models.Product, len(existingProducts))
	for _, p := range existingProducts {
		existingProductsMap[p.Sku] = p
	}

	for _, p := range products {
		if _, ok := existingProductsMap[p.Sku]; !ok {
			return nil, errors.NewProductNotFoundError(p.Sku)
		}
	}

	return existingProductsMap, nil
}

func getProductMapSnapshot(productMap map[uint64]*models.Product) map[uint64]models.Product {
	snapshot := make(map[uint64]models.Product, len(productMap))
	for sku, p := range productMap {
		snapshot[sku] = *p
	}
	return snapshot
}
