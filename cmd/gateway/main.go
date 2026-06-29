package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quarkgate/quarkgate/internal/config"
	"github.com/quarkgate/quarkgate/internal/db"
	"github.com/quarkgate/quarkgate/internal/gateway"
	"github.com/quarkgate/quarkgate/internal/observability"
	"github.com/quarkgate/quarkgate/internal/proxy"
	"github.com/quarkgate/quarkgate/internal/registry"
	qredis "github.com/quarkgate/quarkgate/internal/redis"
	"github.com/quarkgate/quarkgate/internal/store"
	"github.com/quarkgate/quarkgate/internal/vault"
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool,
		"migrations/001_initial.sql",
		"migrations/002_seed_providers.sql",
		"migrations/003_usage_ledger_links.sql",
	); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}

	rdb, err := qredis.Connect(cfg.RedisURL)
	if err != nil {
		log.Error("redis connect", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	st := store.New(pool)
	v, err := vault.New(cfg.VaultKEK)
	if err != nil {
		log.Error("vault", "err", err)
		os.Exit(1)
	}
	vs := vault.NewStore(pool, v)

	reg, err := registry.Load(cfg.DriversPath, cfg.NodePath)
	if err != nil {
		log.Error("registry", "err", err)
		os.Exit(1)
	}

	proxyHandler := proxy.NewHandler(cfg.ConnectTimeout, cfg.StreamIdleSec, cfg.StreamMaxSec, rdb, reg, log)
	proxyHandler.SetBreaker(proxy.NewCircuitBreaker(0.5, 30*time.Second))
	if cfg.BulkheadPerProvider > 0 {
		proxyHandler.SetBulkhead(proxy.NewBulkhead(cfg.BulkheadPerProvider))
	}

	var handler http.Handler = http.HandlerFunc(proxyHandler.ServeHTTP)
	handler = gateway.NewSolvencyMiddleware(st, rdb, cfg.HoldTTLMinutes, 10_000_000).Wrap(handler)
	handler = (&gateway.ScopeMiddleware{}).Wrap(handler)
	handler = gateway.NewIdempotencyMiddleware(rdb).Wrap(handler)
	handler = gateway.NewRouteMiddleware(reg, st, vs, log).Wrap(handler)
	handler = gateway.NewRateLimitMiddleware(rdb).Wrap(handler)
	handler = gateway.NewAuthMiddleware(st, rdb, log, cfg.VaultKEK).Wrap(handler)
	handler = (&gateway.RequestIDMiddleware{}).Wrap(handler)

	mux := http.NewServeMux()
	mux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	mux.Handle("/readyz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		if err := rdb.RDB().Ping(r.Context()).Err(); err != nil {
			http.Error(w, "redis down", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	}))
	mux.Handle("/metrics", observability.Handler())
	mux.Handle("/", handler)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("gateway listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info("shutdown", "active_requests", proxy.ActiveCount())
	deadline := time.Now().Add(30 * time.Second)
	for proxy.ActiveCount() > 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
