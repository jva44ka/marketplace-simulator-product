package service

import (
	"context"
	"fmt"
	"maps"
	"slices"

	domainErrors "github.com/jva44ka/ozon-simulator-go-products/internal/domain/errors"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/model"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/reservation"
)

type ProductRepository interface {
	GetProductBySku(ctx context.Context, sku uint64) (*model.Product, error)
	GetProductsBySkus(ctx context.Context, skus []uint64) ([]*model.Product, error)
	UpdateCount(ctx context.Context, products []*model.Product) error
}

type ReservationRepository interface {
	Insert(ctx context.Context, sku uint64, count uint32) (int64, error)
	GetByIds(ctx context.Context, ids []int64) ([]reservation.Reservation, error)
	DeleteByIds(ctx context.Context, ids []int64) error
}

type ProductService struct {
	productRepository     ProductRepository
	reservationRepository ReservationRepository
}

func NewProductService(productRepository ProductRepository, reservationRepository ReservationRepository) *ProductService {
	return &ProductService{
		productRepository:     productRepository,
		reservationRepository: reservationRepository,
	}
}

type UpdateProductCount struct {
	Sku   uint64
	Delta uint32
}

func (s *ProductService) GetProductBySku(ctx context.Context, sku uint64) (*model.Product, error) {
	product, err := s.productRepository.GetProductBySku(ctx, sku)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetProductBySku: %w", err)
	}

	if product == nil {
		return nil, domainErrors.NewProductNotFoundError(sku)
	}

	return product, nil
}

func (s *ProductService) IncreaseCount(ctx context.Context, products []UpdateProductCount) error {
	existingProductsMap, err := s.validateProductsExist(ctx, products)
	if err != nil {
		return err
	}

	for _, product := range products {
		existingProductsMap[product.Sku].Count += product.Delta
	}

	return s.productRepository.UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap)))
}

// ReserveCount уменьшает доступный count и создаёт записи в reservations.
// Возвращает map[sku → reservation_id].
func (s *ProductService) ReserveCount(ctx context.Context, products []UpdateProductCount) (map[uint64]int64, error) {
	existingProductsMap, err := s.validateProductsExist(ctx, products)
	if err != nil {
		return nil, err
	}

	for _, p := range products {
		existing := existingProductsMap[p.Sku]
		if existing.Count < p.Delta {
			return nil, domainErrors.NewInsufficientProductError(p.Sku, existing.Count, p.Delta)
		}
		existing.Count -= p.Delta
	}

	if err = s.productRepository.UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
		return nil, fmt.Errorf("productRepository.UpdateCount: %w", err)
	}

	reservationIds := make(map[uint64]int64, len(products))
	for _, p := range products {
		id, err := s.reservationRepository.Insert(ctx, p.Sku, p.Delta)
		if err != nil {
			return nil, fmt.Errorf("reservationRepository.Insert: %w", err)
		}
		reservationIds[p.Sku] = id
	}

	return reservationIds, nil
}

// ReleaseReservationByIds возвращает зарезервированный count обратно (вызывается из gRPC).
func (s *ProductService) ReleaseReservationByIds(ctx context.Context, ids []int64) error {
	reservations, err := s.reservationRepository.GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("reservationRepository.GetByIds: %w", err)
	}

	products := make([]UpdateProductCount, len(reservations))
	for i, r := range reservations {
		products[i] = UpdateProductCount{Sku: r.Sku, Delta: r.Count}
	}

	if err = s.ReleaseReservation(ctx, products); err != nil {
		return err
	}

	return s.reservationRepository.DeleteByIds(ctx, ids)
}

// ConfirmReservationByIds подтверждает продажу — count уже был уменьшен при Reserve,
// поэтому только удаляем записи резервации (вызывается из gRPC).
func (s *ProductService) ConfirmReservationByIds(ctx context.Context, ids []int64) error {
	return s.reservationRepository.DeleteByIds(ctx, ids)
}

// ReleaseReservation возвращает count без затрагивания таблицы reservations.
// Используется джобой истечения резерваций.
func (s *ProductService) ReleaseReservation(ctx context.Context, products []UpdateProductCount) error {
	existingProductsMap, err := s.validateProductsExist(ctx, products)
	if err != nil {
		return err
	}

	for _, p := range products {
		existingProductsMap[p.Sku].Count += p.Delta
	}

	return s.productRepository.UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap)))
}

func (s *ProductService) validateProductsExist(
	ctx context.Context,
	products []UpdateProductCount,
) (map[uint64]*model.Product, error) {
	skus := make([]uint64, 0, len(products))
	for _, p := range products {
		skus = append(skus, p.Sku)
	}

	existingProducts, err := s.productRepository.GetProductsBySkus(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetProductsBySkus: %w", err)
	}

	existingProductMap := make(map[uint64]*model.Product, len(existingProducts))
	for _, p := range existingProducts {
		existingProductMap[p.Sku] = p
	}

	for _, p := range products {
		if _, ok := existingProductMap[p.Sku]; !ok {
			return nil, domainErrors.NewProductNotFoundError(p.Sku)
		}
	}

	return existingProductMap, nil
}
