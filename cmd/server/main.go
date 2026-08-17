package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/monoposer/lowcode-database/internal/api"
	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/internal/service"
	"github.com/monoposer/lowcode-database/internal/service/calc"
	"github.com/monoposer/lowcode-database/internal/worker"
	"github.com/monoposer/lowcode-database/pkg/config"
	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
	infraredis "github.com/monoposer/lowcode-database/pkg/infra/redis"
	"github.com/monoposer/lowcode-database/pkg/logger"
	"github.com/monoposer/lowcode-database/pkg/platform/authn"
	"github.com/monoposer/lowcode-database/pkg/platform/cache"
	"github.com/monoposer/lowcode-database/pkg/platform/ratelimit"
	"github.com/monoposer/lowcode-database/pkg/telemetry"
	"github.com/monoposer/lowcode-database/pkg/version"
)

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Tenant-Id, X-Tenant-ID, X-Api-Key, Authorization, X-User-Sub, X-User-Roles, X-User-Role, X-Requested-With, X-Read-Consistency, X-Base-Id, X-Confirm-Dangerous")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withRequestLog(l *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if l != nil {
			l.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}
	})
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	var (
		httpAddr    = flag.String("http-addr", cfg.HTTPAddr, "HTTP JSON API listen address")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()
	if *showVersion {
		fmt.Println(version.String())
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	appLog := logger.New(cfg.LogLevel)

	rdb, err := infraredis.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	if rdb != nil {
		defer rdb.Close()
		if cfg.CacheEnabled {
			appLog.Info("redis connected", "cache", true)
		}
	}

	tenantMgr, err := postgres.NewTenantManager(ctx, cfg, appLog)
	if err != nil {
		log.Fatalf("init tenant manager: %v", err)
	}

	metaCache := cache.New(cfg, rdb)
	shutdownTel, err := telemetry.Init(ctx)
	if err != nil {
		log.Fatalf("telemetry: %v", err)
	}
	defer func() { _ = shutdownTel(context.Background()) }()
	bus := event.Open(cfg, rdb)
	defer bus.Close()
	lcSvc := service.NewLowcodeService(tenantMgr, cfg.MaxRow,
		service.WithCache(metaCache, time.Duration(cfg.CacheTTLSeconds)*time.Second),
		service.WithPGStatStatements(cfg.PGStatStatements),
		service.WithLogger(appLog, time.Duration(cfg.SlowQueryThresholdMS)*time.Millisecond),
		service.WithLimits(cfg),
		service.WithHTTPMiddleware(ratelimit.New(cfg.RateLimitGlobalRPS, cfg.RateLimitTenantRPS).Middleware),
		service.WithEventBus(bus),
	)
	event.StartWebhookDispatcher(ctx, bus, tenantMgr)
	if cfg.PGStatStatements {
		appLog.Info("pg_stat_statements list API enabled", "path", "/v1/admin/pg-stat-statements")
	}
	appLog.Info("logging", "level", cfg.LogLevel)

	go (&worker.IndexMigrate{
		Tenants:  tenantMgr,
		EventBus: bus,
		Interval: 10 * time.Second,
		Timeout:  time.Duration(cfg.IndexBackfillTimeoutSec) * time.Second,
	}).Run(ctx)
	go calc.NewWorker(calc.WorkerConfig{
		Tenants:       tenantMgr,
		Batch:         cfg.CalcWorkerBatch,
		Poll:          time.Duration(cfg.CalcWorkerPollMS) * time.Millisecond,
		Log:           appLog,
		PerTenant:     cfg.CalcTenantConcurrency,
		AlertQueueLen: cfg.CalcAlertQueueLen,
	}).Run(ctx)
	appLog.Info("background workers started", "index_migrate", true, "calc", true, "batch", cfg.CalcWorkerBatch, "poll_ms", cfg.CalcWorkerPollMS)

	mux := http.NewServeMux()
	authnValidator := authn.NewValidator(tenantMgr.MetaPool(), cfg)
	mux.Handle("/v1/", authnValidator.Middleware(api.NewHandler(lcSvc)))
	api.RegisterOpenAPI(mux)
	if st, err := os.Stat("web/playground/dist"); err == nil && st.IsDir() {
		mux.Handle("/playground/", http.StripPrefix("/playground/", http.FileServer(http.Dir("web/playground/dist"))))
	}
	mux.HandleFunc("/health", api.HealthHandler("server"))
	mux.HandleFunc("/", api.RootHandler)

	handler := withCORS(withRequestLog(appLog, mux))

	httpServer := &http.Server{
		Addr:              *httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		appLog.Info("server starting",
			"addr", *httpAddr,
			"version", version.Version,
			"commit", version.Commit,
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	appLog.Info("shutting down")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = httpServer.Shutdown(shutdownCtx)
}
