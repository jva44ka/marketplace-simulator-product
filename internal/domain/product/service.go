package product

import (
	"context"
	"fmt"
	"maps"
	"slices"

	domainErrors "github.com/jva44ka/ozon-simulator-go-products/internal/domain/errors"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/reservation"
)

type Repository interface {
	GetProductBySku(ctx context.Context, sku uint64) (*Product, error)
	GetProductsBySkus(ctx context.Context, skus []uint64) ([]*Product, error)
	UpdateCount(ctx context.Context, products []*Product) error
}

type ReservationRepository interface {
	Insert(ctx context.Context, sku uint64, count uint32) (reservation.Reservation, error)
	GetByIds(ctx context.Context, ids []int64) ([]reservation.Reservation, error)
	DeleteByIds(ctx context.Context, ids []int64) error
}

type Service struct {
	productRepository     Repository
	reservationRepository ReservationRepository
}

func NewService(productRepository Repository, reservationRepository ReservationRepository) *Service {
	return &Service{
		productRepository:     productRepository,
		reservationRepository: reservationRepository,
	}
}

type UpdateCount struct {
	Sku   uint64
	Delta uint32
}

func (s *Service) GetProductBySku(ctx context.Context, sku uint64) (*Product, error) {
	product, err := s.productRepository.GetProductBySku(ctx, sku)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetProductBySku: %w", err)
	}

	if product == nil {
		return nil, domainErrors.NewProductNotFoundError(sku)
	}

	return product, nil
}

func (s *Service) IncreaseCount(ctx context.Context, products []UpdateCount) error {
	existinProductsMap, err := s.validateProductsExist(ctx, products)
	if err != nil {
		return err
	}
	for _, product := range products {
		existinProductsMap[product.Sku].Count += product.Delta
	}
	return s.productRepository.UpdateCount(ctx, slices.Collect(maps.Values(existinProductsMap)))
}

func (s *Service) Reserve(ctx context.Context, products []UpdateCount) (map[uint64]int64, error) {
	existingProductsMap, err := s.validateProductsExist(ctx, products)
	if err != nil {
		return nil, err
	}
	for _, product := range products {
		existingProduct := existingProductsMap[product.Sku]
		if existingProduct.Count < product.Delta {
			return nil, domainErrors.NewInsufficientProductError(product.Sku, existingProduct.Count, product.Delta)
		}
		existingProduct.Count -= product.Delta
	}
	if err = s.productRepository.UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
		return nil, fmt.Errorf("ProductService.Reserve: %w", err)
	}

	reservationIds := make(map[uint64]int64, len(products))
	for _, p := range products {
		rv, err := s.reservationRepository.Insert(ctx, p.Sku, p.Delta)
		if err != nil {
			return nil, fmt.Errorf("ProductService.Reserve: %w", err)
		}
		reservationIds[p.Sku] = rv.Id
	}
	return reservationIds, nil
}

func (s *Service) ReleaseReservations(ctx context.Context, ids []int64) error {
	reservations, err := s.reservationRepository.GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("ProductService.ReleaseReservations: %w", err)
	}
	products := make([]UpdateCount, len(reservations))
	for i, r := range reservations {
		products[i] = UpdateCount{Sku: r.Sku, Delta: r.Count}
	}
	if err = s.ReleaseReservation(ctx, products); err != nil {
		return err
	}
	return s.reservationRepository.DeleteByIds(ctx, ids)
}

func (s *Service) ConfirmReservations(ctx context.Context, ids []int64) error {
	return s.reservationRepository.DeleteByIds(ctx, ids)
}

func (s *Service) ReleaseReservation(ctx context.Context, products []UpdateCount) error {
	existingProductsMap, err := s.validateProductsExist(ctx, products)
	if err != nil {
		return err
	}
	for _, p := range products {
		existingProductsMap[p.Sku].Count += p.Delta
	}
	return s.productRepository.UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap)))
}

func (s *Service) validateProductsExist(ctx context.Context, products []UpdateCount) (map[uint64]*Product, error) {
	skus := make([]uint64, 0, len(products))
	for _, product := range products {
		skus = append(skus, product.Sku)
	}

	existingProducts, err := s.productRepository.GetProductsBySkus(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("ProductService.validateProductsExist: %w", err)
	}

	existingProductsMap := make(map[uint64]*Product, len(existingProducts))
	for _, existingProduct := range existingProducts {
		existingProductsMap[existingProduct.Sku] = existingProduct
	}

	for _, product := range products {
		if _, ok := existingProductsMap[product.Sku]; !ok {
			return nil, domainErrors.NewProductNotFoundError(product.Sku)
		}
	}

	return existingProductsMap, nil
}
