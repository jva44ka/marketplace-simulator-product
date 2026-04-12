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
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/config"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/database"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/kafka"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/metrics"
	"github.com/jva44ka/ozon-simulator-go-products/internal/jobs"
	"github.com/jva44ka/ozon-simulator-go-products/internal/services/product"
	"github.com/jva44ka/ozon-simulator-go-products/internal/services/reservation"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "github.com/jva44ka/ozon-simulator-go-products/internal/app/pb/ozon-simulator-go-products/api/v1/proto"
)

type App struct {
	grpcServer             *grpc.Server
	httpServer             *http.Server
	cfg                    *config.Config
	reservationExpiryJob   *jobs.ReservationExpiryJob
	productEventsOutboxJob *jobs.ProductEventsOutboxJob
	producer               *kafka.ProductEventsProducer
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

	//TODO: вынести парсинг опций в конкретные фабричные методы соответствующих сущностей
	reservationTTL, err := time.ParseDuration(cfg.Jobs.ReservationExpiry.TTL)
	if err != nil {
		return nil, fmt.Errorf("parse reservation.ttl: %w", err)
	}

	reservationJobInterval, err := time.ParseDuration(cfg.Jobs.ReservationExpiry.JobInterval)
	if err != nil {
		return nil, fmt.Errorf("parse reservation.job-interval: %w", err)
	}

	outboxJobInterval, err := time.ParseDuration(cfg.Jobs.ProductEventsOutbox.JobInterval)
	if err != nil {
		return nil, fmt.Errorf("parse outbox_record_builder.job-interval: %w", err)
	}

	kafkaWriteTimeout, err := time.ParseDuration(cfg.Kafka.WriteTimeout)
	if err != nil {
		return nil, fmt.Errorf("parse kafka.write-timeout: %w", err)
	}

	dbMetrics := metrics.NewDbMetrics()
	db := database.NewDBManager(pool, dbMetrics, dbMetrics)
	producer := kafka.NewProductEventsProducer(cfg.Kafka.Brokers, cfg.Kafka.ProductEventsTopic, kafkaWriteTimeout)

	productService := product.NewService(db)
	reservationService := reservation.NewService(db)

	reservationExpiryJob := jobs.NewReservationExpiryJob(
		db.ReservationPgxRepo(),
		reservationService,
		reservationTTL,
		reservationJobInterval,
		cfg.Jobs.ReservationExpiry.Enabled)

	productEventsOutboxJob := jobs.NewProductEventsOutboxJob(
		db,
		producer,
		cfg.Jobs.ProductEventsOutbox.Enabled,
		outboxJobInterval,
		cfg.Jobs.ProductEventsOutbox.BatchSize,
		int32(cfg.Jobs.ProductEventsOutbox.MaxRetries))

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
	grpcService := NewGrpcService(productService, reservationService)

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
		grpcServer:             grpcServer,
		httpServer:             httpServer,
		cfg:                    cfg,
		reservationExpiryJob:   reservationExpiryJob,
		productEventsOutboxJob: productEventsOutboxJob,
		producer:               producer,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	lis, err := net.Listen("tcp", ":"+a.cfg.GrpcServer.Port)
	if err != nil {
		return err
	}

	errGroup, ctx := errgroup.WithContext(ctx)

	errGroup.Go(func() error {
		slog.Info("starting reservation expiry job")
		a.reservationExpiryJob.Run(ctx)

		return nil
	})

	errGroup.Go(func() error {
		slog.Info("starting product events outbox job")
		a.productEventsOutboxJob.Run(ctx)

		return nil
	})

	errGroup.Go(func() error {
		return a.grpcServer.Serve(lis)
	})

	errGroup.Go(func() error {
		return a.httpServer.ListenAndServe()
	})

	errGroup.Go(func() error {
		<-ctx.Done()
		a.grpcServer.Stop()
		return a.httpServer.Shutdown(ctx)
	})

	return errGroup.Wait()
}
