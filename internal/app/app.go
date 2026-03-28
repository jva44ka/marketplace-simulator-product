package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/app/middleware"
	"github.com/jva44ka/ozon-simulator-go-products/internal/data"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/services"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/config"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/jobs"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/kafka"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "github.com/jva44ka/ozon-simulator-go-products/internal/app/pb/ozon-simulator-go-products/api/v1/proto"
)

type App struct {
	grpcServer *grpc.Server
	httpServer *http.Server
	cfg        *config.Config
	job        *jobs.ReservationExpiryJob
	producer   *kafka.Producer
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

	reservationTTL, err := time.ParseDuration(cfg.Reservation.TTL)
	if err != nil {
		return nil, fmt.Errorf("parse reservation.ttl: %w", err)
	}

	jobInterval, err := time.ParseDuration(cfg.Reservation.JobInterval)
	if err != nil {
		return nil, fmt.Errorf("parse reservation.job-interval: %w", err)
	}

	dbMetrics := metrics.NewDbMetrics()
	db := data.NewDBManager(pool, dbMetrics, dbMetrics)
	producer := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ReservationExpiredTopic)

	domainService := services.NewService(db)
	expiryJob := jobs.NewReservationExpiryJob(db.ReservationRepo(), domainService, producer, reservationTTL, jobInterval)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.Panic,
			middleware.ResponseTime(metrics.NewRequestMetrics()),
			middleware.Logger(cfg),
			middleware.StatusCode,
			middleware.Auth(cfg),
			middleware.Validate,
		),
	)
	grpcService := NewGrpcService(domainService)

	pb.RegisterProductsServer(grpcServer, grpcService)

	ctx := context.Background()
	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(
			func(header string) (string, bool) {
				switch strings.ToLower(header) {
				case "x-auth":
					return header, true
				default:
					return runtime.DefaultHeaderMatcher(header)
				}
			},
		))

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
	httpMux.Handle("/", mux)
	httpMux.Handle("/api/", http.StripPrefix(
		"/api/",
		http.FileServer(http.Dir("./swagger/api/v1")),
	))
	httpMux.HandleFunc("/swagger/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, swaggerUiHtml)
	})
	httpMux.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    cfg.HttpServer.Host + ":" + cfg.HttpServer.Port,
		Handler: httpMux,
	}

	return &App{
		grpcServer: grpcServer,
		httpServer: httpServer,
		cfg:        cfg,
		job:        expiryJob,
		producer:   producer,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	go func() {
		slog.Info("starting reservation expiry job")
		a.job.Run(ctx)
	}()

	lis, err := net.Listen("tcp", ":"+a.cfg.GrpcServer.Port)
	if err != nil {
		return err
	}

	go func() {
		a.grpcServer.Serve(lis)
	}()

	return a.httpServer.ListenAndServe()
}
