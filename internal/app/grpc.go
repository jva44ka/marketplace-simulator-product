package app

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/jva44ka/ozon-simulator-go-products/internal/app/gen/ozon-simulator-go-products/api/proto"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ pb.ProductsServer = (*GrpcService)(nil)

type GrpcService struct {
	pb.UnimplementedProductsServer
	ProductService service.ProductService
}

func NewGrpcService(productService *service.ProductService) *GrpcService {
	return &GrpcService{ProductService: *productService}
}

func (s *GrpcService) GetProduct(ctx context.Context, request *pb.GetProductRequest) (resp *pb.GetProductResponse, err error) {
	if request.Sku < 1 {
		return &pb.GetProductResponse{}, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
	}

	product, err := s.ProductService.GetProductBySku(ctx, request.Sku)
	if err != nil {
		return &pb.GetProductResponse{}, fmt.Errorf("error getting product: %w", err)
	}

	response := &pb.GetProductResponse{
		Sku:   product.Sku,
		Name:  product.Name,
		Price: float32(product.Price),
		Count: product.Count,
	}

	return response, nil
}

func (s *GrpcService) IncreaseStock(
	_ context.Context,
	_ *pb.IncreaseStockRequest,
) (*pb.IncreaseStockResponse, error) {

	return nil, errors.New("unimplemented IncreaseStock")
}

func (s *GrpcService) DecreaseStock(
	_ context.Context,
	_ *pb.DecreaseStockRequest,
) (*pb.DecreaseStockResponse, error) {

	return nil, errors.New("unimplemented DecreaseStock")
}
