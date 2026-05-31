package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/solat/lowcode-database/internal/api"
	"github.com/solat/lowcode-database/internal/cache"
	"github.com/solat/lowcode-database/internal/config"
	"github.com/solat/lowcode-database/internal/db"
	"github.com/solat/lowcode-database/internal/logger"
	"github.com/solat/lowcode-database/internal/metrics"
	"github.com/solat/lowcode-database/internal/redisclient"
	"github.com/solat/lowcode-database/internal/service"
	"github.com/solat/lowcode-database/internal/webhook"
)

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Tenant-Id, X-Tenant-ID, X-Requested-With")

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
		httpAddr = flag.String("http-addr", cfg.HTTPAddr, "HTTP JSON API listen address")
	)
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	appLog := logger.New(cfg.LogLevel)

	rdb, err := redisclient.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	if rdb != nil {
		defer rdb.Close()
		if cfg.CacheEnabled {
			appLog.Info("redis connected", "cache", true)
		}
	}

	tenantMgr, err := db.NewTenantManager(ctx, cfg)
	if err != nil {
		log.Fatalf("init tenant manager: %v", err)
	}

	hooks := webhook.NewDispatcher(tenantMgr)
	metaCache := cache.New(cfg, rdb)
	dsMetrics := metrics.New(cfg, rdb)
	if cfg.MetricsBackend != "" && cfg.MetricsBackend != "noop" {
		appLog.Info("metrics enabled", "backend", cfg.MetricsBackend, "window", cfg.MetricsWindowSize)
	}

	lcSvc := service.NewLowcodeService(tenantMgr, cfg.MaxRow, hooks,
		service.WithCache(metaCache, time.Duration(cfg.CacheTTLSeconds)*time.Second),
		service.WithMetrics(dsMetrics),
		service.WithLogger(appLog, time.Duration(cfg.SlowQueryThresholdMS)*time.Millisecond),
	)

	mux := http.NewServeMux()
	mux.Handle("/v1/", api.NewHandler(lcSvc))
	if cfg.MetricsBackend == "prometheus" {
		mux.Handle("/metrics", promhttp.Handler())
	}
	api.RegisterOpenAPI(mux)
	mux.HandleFunc("/", api.RootHandler)

	handler := withCORS(withRequestLog(appLog, mux))

	httpServer := &http.Server{
		Addr:              *httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		appLog.Info("server starting", "addr", *httpAddr)
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
