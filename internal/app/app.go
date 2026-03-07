package app

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jva44ka/ozon-simulator-go-products/swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "github.com/jva44ka/ozon-simulator-go-products/internal/app/gen/ozon-simulator-go-products/api/proto"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/repository"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/service"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/config"

	httpSwagger "github.com/swaggo/http-swagger"
)

type App struct {
	grpcServer *grpc.Server
	httpServer *http.Server
	cfg        *config.Config
}

func NewApp(cfg *config.Config) (*App, error) {
	pool, err := pgxpool.New(context.Background(), fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
	))
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}

	repo := repository.NewPgxRepository(pool)
	domainService := service.NewProductService(repo)

	grpcServer := grpc.NewServer()
	grpcService := NewGrpcService(domainService)

	pb.RegisterProductsServer(grpcServer, grpcService)

	ctx := context.Background()
	mux := runtime.NewServeMux()

	err = pb.RegisterProductsHandlerFromEndpoint(
		ctx,
		mux,
		cfg.GrpcServer.Host+":"+cfg.GrpcServer.Port,
		[]grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	)
	if err != nil {
		return nil, err
	}

	reflection.Register(grpcServer)

	httpMux := http.NewServeMux()
	// grpc gateway
	httpMux.Handle("/", mux)
	// swagger json
	httpMux.Handle("/api/", http.StripPrefix(
		"/api/",
		http.FileServer(http.Dir("./swagger/api")),
	))
	// swagger UI
	httpMux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/api/products.swagger.json"),
	))

	httpServer := &http.Server{
		Addr:    cfg.HttpServer.Host + ":" + cfg.HttpServer.Port,
		Handler: httpMux,
	}

	return &App{
		grpcServer: grpcServer,
		httpServer: httpServer,
		cfg:        cfg,
	}, nil
}

func (a *App) Run() error {

	lis, err := net.Listen("tcp", ":"+a.cfg.GrpcServer.Port)
	if err != nil {
		return err
	}

	go func() {
		a.grpcServer.Serve(lis)
	}()

	return a.httpServer.ListenAndServe()
}
