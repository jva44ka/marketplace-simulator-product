package service

import (
	"context"
	"fmt"

	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/model"
)

type ProductRepository interface {
	GetProductBySku(ctx context.Context, sku uint64) (*model.Product, error)
	GetProductsBySkus(ctx context.Context, skus []uint64) ([]*model.Product, error)
	IncreaseCount(ctx context.Context, stocks []UpdateProductCount) error
	DecreaseCount(ctx context.Context, stocks []UpdateProductCount) error
}

type ProductService struct {
	productRepository ProductRepository
}

func NewProductService(productRepository ProductRepository) *ProductService {
	return &ProductService{productRepository: productRepository}
}

type UpdateProductCount struct {
	Sku   uint64
	Delta uint32
}

func (s *ProductService) GetProductBySku(ctx context.Context, sku uint64) (*model.Product, error) {
	product, err := s.productRepository.GetProductBySku(ctx, sku)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetProductBySku :%w", err)
	}

	return product, nil
}

func (s *ProductService) IncreaseStock(ctx context.Context, products []UpdateProductCount) error {
	if _, err := s.validateProductsExist(ctx, products); err != nil {
		return err
	}

	if err := s.productRepository.IncreaseCount(ctx, products); err != nil {
		return fmt.Errorf("productRepository.IncreaseCount: %w", err)
	}
	return nil
}

func (s *ProductService) DecreaseStock(ctx context.Context, products []UpdateProductCount) error {
	existingProducts, err := s.validateProductsExist(ctx, products)
	if err != nil {
		return err
	}

	for _, product := range products {
		existingProduct := existingProducts[product.Sku]
		if existingProduct.Count < product.Delta {
			return fmt.Errorf(
				"insufficient product for sku %d: have %d, want %d",
				product.Sku,
				existingProduct.Count,
				product.Delta)
		}
	}

	if err = s.productRepository.DecreaseCount(ctx, products); err != nil {
		return fmt.Errorf("productRepository.DecreaseCount: %w", err)
	}

	return nil
}

func (s *ProductService) validateProductsExist(
	ctx context.Context,
	products []UpdateProductCount) (map[uint64]*model.Product, error) {
	skus := make([]uint64, 0, len(products))
	for _, stock := range products {
		skus = append(skus, stock.Sku)
	}

	existingProducts, err := s.productRepository.GetProductsBySkus(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetProductsBySkus: %w", err)
	}

	existingProductMap := make(map[uint64]*model.Product, len(existingProducts))
	for _, p := range existingProducts {
		existingProductMap[p.Sku] = p
	}

	for _, product := range products {
		if _, ok := existingProductMap[product.Sku]; !ok {
			return nil, fmt.Errorf("product not found for sku %d", product.Sku)
		}
	}

	return existingProductMap, nil
}
