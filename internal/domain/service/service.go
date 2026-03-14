package service

import (
	"context"
	"fmt"

	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/model"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/repository"
)

type ProductRepository interface {
	GetProductBySku(_ context.Context, sku uint64) (*model.Product, error)
	IncreaseCount(ctx context.Context, products []repository.UpdateProductCount) error
	DecreaseCount(ctx context.Context, products []repository.UpdateProductCount) error
}

type ProductService struct {
	productRepository ProductRepository
}

func NewProductService(productRepository ProductRepository) *ProductService {
	return &ProductService{productRepository: productRepository}
}

func (s *ProductService) GetProductBySku(ctx context.Context, sku uint64) (*model.Product, error) {
	product, err := s.productRepository.GetProductBySku(ctx, sku)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetProductBySku :%w", err)
	}

	return product, nil
}

func (s *ProductService) IncreaseStock(ctx context.Context, items []repository.UpdateProductCount) error {
	batches := make([]repository.UpdateProductCount, 0, len(items))
	for _, item := range items {
		batches = append(batches, repository.UpdateProductCount{
			Sku:   item.Sku,
			Delta: item.Delta,
		})
	}
	if err := s.productRepository.IncreaseCount(ctx, batches); err != nil {
		return fmt.Errorf("productRepository.IncreaseCount: %w", err)
	}
	return nil
}

func (s *ProductService) DecreaseStock(ctx context.Context, items []repository.UpdateProductCount) error {
	batches := make([]repository.UpdateProductCount, 0, len(items))
	for _, item := range items {
		batches = append(batches, repository.UpdateProductCount{
			Sku:   item.Sku,
			Delta: item.Delta,
		})
	}
	if err := s.productRepository.DecreaseCount(ctx, batches); err != nil {
		return fmt.Errorf("productRepository.DecreaseCount: %w", err)
	}
	return nil
}
