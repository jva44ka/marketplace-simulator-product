package product

import (
	"context"
	"fmt"

	"github.com/jva44ka/ozon-simulator-go-products/internal/errors"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
	"github.com/jva44ka/ozon-simulator-go-products/internal/services"
)

type Service struct {
	db services.DBManager
}

func NewService(db services.DBManager) *Service {
	return &Service{db: db}
}

type UpdateCount struct {
	Sku   uint64
	Delta uint32
}

func validateProductsExist(
	ctx context.Context,
	products []UpdateCount,
	repo services.ProductRepository) (map[uint64]*models.Product, error) {
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
