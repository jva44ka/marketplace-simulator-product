package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/app/middleware"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/database"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/kafka"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/metrics"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/tracing"
	"github.com/jva44ka/marketplace-simulator-product/internal/jobs"
	"github.com/jva44ka/marketplace-simulator-product/internal/services/product"
	"github.com/jva44ka/marketplace-simulator-product/internal/services/reservation"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "github.com/jva44ka/marketplace-simulator-product/internal/app/pb/marketplace-simulator-product/api/v1/proto"
)

type App struct {
	grpcServer             *grpc.Server
	httpServer             *http.Server
	cfg                    *config.Config
	reservationExpiryJob   *jobs.ReservationExpiryJob
	productEventsOutboxJob *jobs.ProductEventsOutboxJob
	metricCollectorJob     *jobs.MetricCollectorJob
	producer               *kafka.ProductEventsProducer
	tracingCloser          func(context.Context) error
}

func NewApp(ctx context.Context, cfg *config.Config) (*App, error) {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
	)
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}
	poolConfig.ConnConfig.Tracer = tracing.NewPgxTracer()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}

	var tracingCloser func(context.Context) error
	if cfg.Tracing.Enabled {
		tracingCloser, err = tracing.InitTracer(ctx, "products", cfg.Tracing.OtlpEndpoint)
		if err != nil {
			return nil, fmt.Errorf("tracing.InitTracer: %w", err)
		}
	} else {
		tracingCloser = func(context.Context) error {
			return nil
		}
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

	outboxIdleInterval, err := time.ParseDuration(cfg.Jobs.ProductEventsOutbox.IdleInterval)
	if err != nil {
		return nil, fmt.Errorf("parse product-events-outbox.idle-interval: %w", err)
	}

	outboxActiveInterval, err := time.ParseDuration(cfg.Jobs.ProductEventsOutbox.ActiveInterval)
	if err != nil {
		return nil, fmt.Errorf("parse product-events-outbox.active-interval: %w", err)
	}

	outboxMonitorInterval, err := time.ParseDuration(cfg.Jobs.ProductEventsOutboxMonitor.JobInterval)
	if err != nil {
		return nil, fmt.Errorf("parse outbox-monitor.job-interval: %w", err)
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

	outboxMetrics := metrics.NewOutboxMetrics()
	outboxJob := jobs.NewProductEventsOutboxJob(
		db,
		producer,
		outboxMetrics,
		cfg.Jobs.ProductEventsOutbox.Enabled,
		outboxIdleInterval,
		outboxActiveInterval,
		cfg.Jobs.ProductEventsOutbox.BatchSize,
		int32(cfg.Jobs.ProductEventsOutbox.MaxRetries))

	metricCollectorMetrics := metrics.NewMetricCollectorMetrics()
	metricCollectorJob := jobs.NewMetricCollectorJob(
		db.ProductEventsOutboxRepo(),
		pool,
		metricCollectorMetrics,
		cfg.Jobs.ProductEventsOutboxMonitor.Enabled,
		outboxMonitorInterval)

	interceptors := []grpc.UnaryServerInterceptor{
		middleware.Panic,
		middleware.ResponseTime(metrics.NewRequestMetrics()),
		middleware.Logger(cfg),
		middleware.StatusCode,
		middleware.Auth(cfg),
		middleware.Validate,
	}
	if cfg.RateLimiter.Enabled {
		limiter := rate.NewLimiter(rate.Limit(cfg.RateLimiter.RPS), cfg.RateLimiter.Burst)
		interceptors = append([]grpc.UnaryServerInterceptor{middleware.RateLimit(limiter)}, interceptors...)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(interceptors...),
	)
	grpcService := NewGrpcService(productService, reservationService)

	pb.RegisterProductsServer(grpcServer, grpcService)

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
		productEventsOutboxJob: outboxJob,
		metricCollectorJob:     metricCollectorJob,
		producer:               producer,
		tracingCloser:          tracingCloser,
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
		slog.Info("starting metric collector job")
		a.metricCollectorJob.Run(ctx)

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

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		a.grpcServer.GracefulStop()
		httpShutdownErr := a.httpServer.Shutdown(shutdownCtx)
		tracingCloseErr := a.tracingCloser(shutdownCtx)

		return errors.Join(httpShutdownErr, tracingCloseErr)
	})

	return errGroup.Wait()
}
