package app

import (
	"context"
	"fmt"

	pb "github.com/jva44ka/marketplace-simulator-product/internal/app/pb/marketplace-simulator-product/api/v1/proto"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	ucProduct "github.com/jva44ka/marketplace-simulator-product/internal/usecases/product"
	ucReservation "github.com/jva44ka/marketplace-simulator-product/internal/usecases/reservation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ pb.ProductsServer = (*GrpcService)(nil)

type GrpcService struct {
	pb.UnimplementedProductsServer
	getProduct    *ucProduct.GetProductUseCase
	increaseCount *ucProduct.IncreaseCountUseCase
	reserve       *ucReservation.ReserveUseCase
	release       *ucReservation.ReleaseUseCase
	confirm       *ucReservation.ConfirmUseCase
}

func NewGrpcService(
	getProduct *ucProduct.GetProductUseCase,
	increaseCount *ucProduct.IncreaseCountUseCase,
	reserve *ucReservation.ReserveUseCase,
	release *ucReservation.ReleaseUseCase,
	confirm *ucReservation.ConfirmUseCase,
) *GrpcService {
	return &GrpcService{
		getProduct:    getProduct,
		increaseCount: increaseCount,
		reserve:       reserve,
		release:       release,
		confirm:       confirm,
	}
}

func (s *GrpcService) GetProduct(ctx context.Context, request *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	if request.Sku < 1 {
		return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
	}

	p, err := s.getProduct.Execute(ctx, request.Sku, request.TransactionId)
	if err != nil {
		return nil, fmt.Errorf("GrpcService.GetProduct: %w", err)
	}

	return productToResponse(p), nil
}

func (s *GrpcService) IncreaseProductCount(
	ctx context.Context,
	request *pb.IncreaseProductCountRequest) (*pb.IncreaseProductCountResponse, error) {
	if len(request.Products) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "products must not be empty")
	}

	seenSkus := make(map[uint64]struct{}, len(request.Products))
	products := make([]ucProduct.UpdateCount, 0, len(request.Products))
	for _, stock := range request.Products {
		if stock.Sku < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
		}
		if _, exists := seenSkus[stock.Sku]; exists {
			return nil, status.Errorf(codes.InvalidArgument, "duplicate sku %d in request", stock.Sku)
		}
		seenSkus[stock.Sku] = struct{}{}
		products = append(products, ucProduct.UpdateCount{
			Sku:   stock.Sku,
			Delta: stock.Count,
		})
	}

	if err := s.increaseCount.Execute(ctx, products); err != nil {
		return nil, fmt.Errorf("GrpcService.IncreaseProductCount: %w", err)
	}

	return &pb.IncreaseProductCountResponse{}, nil
}

func (s *GrpcService) ReserveProduct(
	ctx context.Context,
	request *pb.ReserveProductRequest) (*pb.ReserveProductResponse, error) {
	if len(request.Products) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "products must not be empty")
	}

	seenSkus := make(map[uint64]struct{}, len(request.Products))
	items := make([]ucReservation.ReserveItem, 0, len(request.Products))
	for _, stock := range request.Products {
		if stock.Sku < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
		}
		if _, exists := seenSkus[stock.Sku]; exists {
			return nil, status.Errorf(codes.InvalidArgument, "duplicate sku %d in request", stock.Sku)
		}
		seenSkus[stock.Sku] = struct{}{}
		items = append(items, ucReservation.ReserveItem{
			Sku:   stock.Sku,
			Delta: stock.Count,
		})
	}

	reservationIds, err := s.reserve.Execute(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("GrpcService.ReserveProduct: %w", err)
	}

	results := make([]*pb.ReserveProductResponse_ReservationResult, 0, len(reservationIds))
	for sku, id := range reservationIds {
		results = append(results, &pb.ReserveProductResponse_ReservationResult{
			ReservationId: id,
			Sku:           sku,
		})
	}

	return &pb.ReserveProductResponse{Results: results}, nil
}

func (s *GrpcService) ReleaseReservation(
	ctx context.Context,
	request *pb.ReleaseReservationRequest) (*pb.ReleaseReservationResponse, error) {
	if len(request.ReservationIds) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "reservation_ids must not be empty")
	}

	if err := s.release.Execute(ctx, request.ReservationIds); err != nil {
		return nil, fmt.Errorf("GrpcService.ReleaseReservation: %w", err)
	}

	return &pb.ReleaseReservationResponse{}, nil
}

func (s *GrpcService) ConfirmReservation(
	ctx context.Context,
	request *pb.ConfirmReservationRequest) (*pb.ConfirmReservationResponse, error) {
	if len(request.ReservationIds) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "reservation_ids must not be empty")
	}

	if err := s.confirm.Execute(ctx, request.ReservationIds); err != nil {
		return nil, fmt.Errorf("GrpcService.ConfirmReservation: %w", err)
	}

	return &pb.ConfirmReservationResponse{}, nil
}

// --- helpers ---

func productToResponse(p *models.Product) *pb.GetProductResponse {
	return &pb.GetProductResponse{
		Sku:           p.Sku,
		Name:          p.Name,
		Price:         p.Price,
		Count:         p.Count - p.ReservedCount,
		TransactionId: p.TransactionId,
	}
}
