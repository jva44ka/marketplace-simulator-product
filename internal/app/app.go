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
	cacheProduct "github.com/jva44ka/marketplace-simulator-product/internal/infra/cache/product"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/database"
	etcdPkg "github.com/jva44ka/marketplace-simulator-product/internal/infra/etcd"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/kafka"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/metrics"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/tracing"
	"github.com/jva44ka/marketplace-simulator-product/internal/jobs"
	"github.com/jva44ka/marketplace-simulator-product/internal/services/product"
	"github.com/jva44ka/marketplace-simulator-product/internal/services/reservation"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
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
	cfg                    *config.Config // начальный yaml-конфиг (для boot-параметров)
	cfgStore               *config.ConfigStore
	reservationExpiryJob   *jobs.ReservationExpiryJob
	productEventsOutboxJob *jobs.ProductEventsOutboxJob
	cacheUpdateOutboxJob   *jobs.CacheUpdateOutboxJob
	metricCollectorJob     *jobs.MetricCollectorJob
	producer               *kafka.ProductEventsProducer
	tracingCloser          func(context.Context) error
	etcdClient             *clientv3.Client // nil если etcd не настроен
	etcdConfigKey          string
	applyDynamicConfig     func(*config.Config)
}

func NewApp(ctx context.Context, cfg *config.Config) (*App, error) {
	cfgStore := config.NewConfigStore(cfg)

	var etcdClient *clientv3.Client
	if cfg.Etcd.Enabled {
		var err error
		etcdClient, err = etcdPkg.NewClient(cfg.Etcd)
		if err != nil {
			slog.Warn("etcd: failed to connect, using yaml defaults", "err", err)
			etcdClient = nil
		} else {
			initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			if etcdCfg, found, err := etcdPkg.ReadFromEtcd(initCtx, etcdClient, cfg.Etcd.ConfigKey); err != nil {
				slog.Warn("etcd: failed to read config, using yaml defaults", "err", err)
			} else if found {
				cfgStore.Store(etcdCfg)
				slog.Info("etcd: loaded config from etcd")
			} else {
				// Первый старт: seeding
				if err := etcdPkg.SeedIfAbsent(initCtx, etcdClient, cfg.Etcd.ConfigKey, cfg); err != nil {
					slog.Warn("etcd: failed to seed config", "err", err)
				}
			}
		}
	}

	// Используем актуальный конфиг (из etcd или yaml)
	currentCfg := cfgStore.Load()

	// --- Инфраструктура (restart-required поля) ---
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		currentCfg.Database.User,
		currentCfg.Database.Password,
		currentCfg.Database.Host,
		currentCfg.Database.Port,
		currentCfg.Database.Name,
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
	if currentCfg.Tracing.Enabled {
		tracingCloser, err = tracing.InitTracer(ctx, "products", currentCfg.Tracing.OtlpEndpoint)
		if err != nil {
			return nil, fmt.Errorf("tracing.InitTracer: %w", err)
		}
	} else {
		tracingCloser = func(context.Context) error { return nil }
	}

	kafkaWriteTimeout, err := time.ParseDuration(currentCfg.Kafka.WriteTimeout)
	if err != nil {
		return nil, fmt.Errorf("parse kafka.write-timeout: %w", err)
	}

	dbMetrics := metrics.NewDbMetrics()
	rawProductRepo := database.NewProductPgxRepository(pool, dbMetrics)
	reservationRepo := database.NewReservationPgxRepository(pool, dbMetrics)
	productEventsOutboxRepo := database.NewProductEventsOutboxRepository(pool)
	cacheUpdateOutboxRepo := database.NewCacheUpdateOutboxRepository(pool)
	productTransactor := database.NewProductServiceTransactor(pool, rawProductRepo, productEventsOutboxRepo, cacheUpdateOutboxRepo)
	reservationTransactor := database.NewReservationServiceTransactor(pool, rawProductRepo, reservationRepo, productEventsOutboxRepo, cacheUpdateOutboxRepo)

	producer := kafka.NewProductEventsProducer(currentCfg.Kafka.Brokers, currentCfg.Kafka.ProductEventsTopic, kafkaWriteTimeout)

	// --- Redis cache (optional; graceful degradation if nil or unavailable) ---
	cacheMetrics := metrics.NewCacheMetrics()

	var productCache *cacheProduct.ProductCache
	if currentCfg.Redis.Enabled {
		cacheTTL, err := time.ParseDuration(currentCfg.Redis.TTL)
		if err != nil {
			slog.Warn("cache: invalid ttl, using 5m", "err", err)
			cacheTTL = 5 * time.Minute
		}
		redisClient := redis.NewClient(&redis.Options{Addr: currentCfg.Redis.RedisAddr})
		productCache = cacheProduct.NewProductCache(redisClient, cacheTTL, cacheMetrics)
		slog.Info("cache: Redis connected", "addr", currentCfg.Redis.RedisAddr)
	}

	// Wrap the plain product repository with the cache decorator so that all
	// read paths (GetBySku) benefit from Redis without any cache logic leaking
	// into the service or transport layers.
	cachedProductRepo := cacheProduct.NewCachedProductRepository(rawProductRepo, productCache, cacheMetrics)

	productService := product.NewService(productTransactor, cachedProductRepo)
	reservationService := reservation.NewService(reservationTransactor, cachedProductRepo, reservationRepo)

	reservationExpiryJob := jobs.NewReservationExpiryJob(
		reservationRepo,
		reservationService,
		cfgStore,
	)

	outboxMetrics := metrics.NewProductEventOutboxMetrics()
	outboxJob := jobs.NewProductEventsOutboxJob(
		productEventsOutboxRepo,
		producer,
		outboxMetrics,
		cfgStore,
	)

	cacheUpdateOutboxMetrics := metrics.NewCacheUpdateOutboxMetrics()
	cacheUpdateOutboxJob := jobs.NewCacheUpdateOutboxJob(
		cacheUpdateOutboxRepo,
		rawProductRepo, // must bypass cache: the job's job is to populate it
		productCache,
		cacheUpdateOutboxMetrics,
		cfgStore,
	)

	metricCollectorMetrics := metrics.NewMetricCollectorMetrics()
	metricCollectorJob := jobs.NewMetricCollectorJob(
		productEventsOutboxRepo,
		cacheUpdateOutboxRepo,
		pool,
		metricCollectorMetrics,
		cfgStore,
	)

	// --- Rate limiter (всегда создаём, middleware читает текущее состояние) ---
	var limiter *rate.Limiter
	if currentCfg.RateLimiter.Enabled {
		limiter = rate.NewLimiter(rate.Limit(currentCfg.RateLimiter.RPS), currentCfg.RateLimiter.Burst)
	} else {
		limiter = rate.NewLimiter(rate.Inf, 0)
	}

	// --- applyDynamicConfig: вызывается при каждом изменении в etcd ---
	applyDynamicConfig := func(newCfg *config.Config) {
		if newCfg.RateLimiter.Enabled {
			limiter.SetLimit(rate.Limit(newCfg.RateLimiter.RPS))
			limiter.SetBurst(newCfg.RateLimiter.Burst)
		} else {
			limiter.SetLimit(rate.Inf)
		}
		slog.Info("dynamic config applied",
			"rate-limiter.enabled", newCfg.RateLimiter.Enabled,
			"rate-limiter.rps", newCfg.RateLimiter.RPS,
		)
	}

	// --- gRPC interceptors ---
	interceptors := []grpc.UnaryServerInterceptor{
		middleware.RateLimit(limiter),
		middleware.Panic,
		middleware.ResponseTime(metrics.NewRequestMetrics()),
		middleware.Logger(cfgStore),
		middleware.StatusCode,
		middleware.Auth(cfgStore),
		middleware.Validate,
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

	etcdConfigKey := ""
	if cfg.Etcd.Enabled {
		etcdConfigKey = cfg.Etcd.ConfigKey
	}

	return &App{
		grpcServer:             grpcServer,
		httpServer:             httpServer,
		cfg:                    cfg,
		cfgStore:               cfgStore,
		reservationExpiryJob:   reservationExpiryJob,
		productEventsOutboxJob: outboxJob,
		cacheUpdateOutboxJob:   cacheUpdateOutboxJob,
		metricCollectorJob:     metricCollectorJob,
		producer:               producer,
		tracingCloser:          tracingCloser,
		etcdClient:             etcdClient,
		etcdConfigKey:          etcdConfigKey,
		applyDynamicConfig:     applyDynamicConfig,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	lis, err := net.Listen("tcp", ":"+a.cfg.GrpcServer.Port)
	if err != nil {
		return err
	}

	errGroup, ctx := errgroup.WithContext(ctx)

	// etcd watcher
	if a.etcdClient != nil {
		errGroup.Go(func() error {
			etcdPkg.Watch(ctx, a.etcdClient, a.etcdConfigKey, a.cfgStore, a.applyDynamicConfig)
			return nil
		})
	}

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
		slog.Info("starting cache update outbox job")
		a.cacheUpdateOutboxJob.Run(ctx)
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

		var etcdCloseErr error
		if a.etcdClient != nil {
			etcdCloseErr = a.etcdClient.Close()
		}

		return errors.Join(httpShutdownErr, tracingCloseErr, etcdCloseErr)
	})

	return errGroup.Wait()
}
