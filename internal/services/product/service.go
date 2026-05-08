package product

import (
	"context"
	"fmt"

	"github.com/jva44ka/marketplace-simulator-product/internal/errors"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type Service struct {
	transactor Transactor
	products   ReadProductRepository
}

func NewService(
	transactor Transactor,
	products ReadProductRepository,
) *Service {
	return &Service{
		transactor: transactor,
		products:   products,
	}
}

type UpdateCount struct {
	Sku   uint64
	Delta uint32
}

func validateProductsExist(
	ctx context.Context,
	products []UpdateCount,
	repo ReadProductRepository,
) (map[uint64]*models.Product, error) {
	skus := make([]uint64, 0, len(products))
	for _, product := range products {
		skus = append(skus, product.Sku)
	}

	existingProducts, err := repo.GetBySkus(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("ProductService.validateProductsExist: %w", err)
	}

	existingProductsMap := make(map[uint64]*models.Product, len(existingProducts))
	for _, existingProduct := range existingProducts {
		existingProductsMap[existingProduct.Sku] = existingProduct
	}

	for _, product := range products {
		if _, ok := existingProductsMap[product.Sku]; !ok {
			return nil, errors.NewProductNotFoundError(product.Sku)
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
