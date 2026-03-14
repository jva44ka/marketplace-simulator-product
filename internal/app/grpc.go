package app

import (
	"context"

	pb "github.com/jva44ka/ozon-simulator-go-products/internal/app/gen/ozon-simulator-go-products/api/v1/proto"
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

func (s *GrpcService) GetProduct(ctx context.Context, request *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	if request.Sku < 1 {
		return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
	}

	product, err := s.ProductService.GetProductBySku(ctx, request.Sku)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error getting product: %v", err)
	}

	return &pb.GetProductResponse{
		Sku:   product.Sku,
		Name:  product.Name,
		Price: product.Price,
		Count: product.Count,
	}, nil
}

func (s *GrpcService) IncreaseStock(ctx context.Context, request *pb.IncreaseStockRequest) (*pb.IncreaseStockResponse, error) {
	if len(request.Stocks) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "stocks must not be empty")
	}

	stocks := make([]service.UpdateProductCount, 0, len(request.Stocks))
	for _, stock := range request.Stocks {
		if stock.Sku < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
		}
		stocks = append(stocks, service.UpdateProductCount{
			Sku:   stock.Sku,
			Delta: stock.Count,
		})
	}

	if err := s.ProductService.IncreaseStock(ctx, stocks); err != nil {
		return nil, status.Errorf(codes.Internal, "error increasing stock: %v", err)
	}

	return &pb.IncreaseStockResponse{}, nil
}

func (s *GrpcService) DecreaseStock(ctx context.Context, request *pb.DecreaseStockRequest) (*pb.DecreaseStockResponse, error) {
	if len(request.Stocks) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "stocks must not be empty")
	}

	stocks := make([]service.UpdateProductCount, 0, len(request.Stocks))
	for _, stock := range request.Stocks {
		if stock.Sku < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
		}
		stocks = append(stocks, service.UpdateProductCount{
			Sku:   stock.Sku,
			Delta: stock.Count,
		})
	}

	if err := s.ProductService.DecreaseStock(ctx, stocks); err != nil {
		return nil, status.Errorf(codes.Internal, "error decreasing stock: %v", err)
	}

	return &pb.DecreaseStockResponse{}, nil
}
