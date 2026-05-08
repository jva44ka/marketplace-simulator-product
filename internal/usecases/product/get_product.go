package product

import (
	"context"
	"fmt"

	"github.com/jva44ka/marketplace-simulator-product/internal/errors"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type GetProductUseCase struct {
	products ReadProductRepository
}

func NewGetProductUseCase(products ReadProductRepository) *GetProductUseCase {
	return &GetProductUseCase{products: products}
}

func (uc *GetProductUseCase) Execute(ctx context.Context, sku uint64, txId *uint32) (*models.Product, error) {
	product, err := uc.products.GetBySku(ctx, sku, txId)
	if err != nil {
		return nil, fmt.Errorf("productRepository.Execute: %w", err)
	}
	if product == nil {
		return nil, errors.NewProductNotFoundError(sku)
	}

	if product.ReservedCount > 0 {
		if product.Count >= product.ReservedCount {
			//TODO: резерваций меньше чем оставшихся продуктов
		} else {
			//TODO: резерваций больше чем оставшихся продуктов, алертим
		}
	}

	return product, nil
}
