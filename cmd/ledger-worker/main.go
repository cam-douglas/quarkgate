package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"github.com/quarkgate/quarkgate/internal/config"
	"github.com/quarkgate/quarkgate/internal/db"
	"github.com/quarkgate/quarkgate/internal/metering"
	"github.com/quarkgate/quarkgate/internal/models"
	"github.com/quarkgate/quarkgate/internal/observability"
	"github.com/quarkgate/quarkgate/internal/registry"
	qredis "github.com/quarkgate/quarkgate/internal/redis"
	"github.com/quarkgate/quarkgate/internal/store"
)

const consumerGroup = "ledger-workers"

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if len(os.Args) > 1 && os.Args[1] == "--replay-dlq" {
		replayDLQ(cfg, log)
		return
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
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
		log.Error("redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	if err := rdb.CreateConsumerGroup(ctx, consumerGroup); err != nil {
		log.Error("consumer group", "err", err)
		os.Exit(1)
	}

	st := store.New(pool)
	reg, err := registry.Load(cfg.DriversPath, cfg.NodePath)
	if err != nil {
		log.Error("registry", "err", err)
		os.Exit(1)
	}

	worker := &metering.Worker{
		Store:    st,
		Redis:    rdb,
		Registry: reg,
		Config:   cfg,
		Log:      log,
	}

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", observability.Handler())
		log.Info("worker metrics", "addr", cfg.WorkerMetricsAddr)
		if err := http.ListenAndServe(cfg.WorkerMetricsAddr, mux); err != nil {
			log.Error("metrics server", "err", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go runSweeper(ctx, st, rdb, log, cfg.HoldTTLMinutes)
	go runReconciliation(ctx, st, rdb, log, cfg.ReconcileAutoFix)

	for {
		select {
		case <-sig:
			return
		default:
			processBatch(ctx, worker, rdb, cfg, log)
		}
	}
}

func processBatch(ctx context.Context, worker *metering.Worker, rdb *qredis.Client, cfg config.Config, log *slog.Logger) {
	pending, _ := rdb.AutoClaimMeterEvents(ctx, consumerGroup, "worker-1", 30*time.Second, 10)
	for _, msg := range pending {
		handleMessage(ctx, worker, rdb, cfg, log, msg)
	}

	messages, err := rdb.ReadMeterEvents(ctx, consumerGroup, "worker-1", 10, 2*time.Second)
	if err != nil {
		if err.Error() != "redis: nil" {
			log.Error("read events", "err", err)
		}
		return
	}
	for _, msg := range messages {
		handleMessage(ctx, worker, rdb, cfg, log, msg)
	}
}

func handleMessage(ctx context.Context, worker *metering.Worker, rdb *qredis.Client, cfg config.Config, log *slog.Logger, msg goredis.XMessage) {
	if err := worker.ProcessEvent(ctx, msg.Values); err != nil {
		deliveries := rdb.MessageDeliveries(ctx, consumerGroup, msg.ID)
		log.Error("process", "err", err, "id", msg.ID, "deliveries", deliveries)
		if deliveries >= int64(cfg.WorkerMaxRetries) {
			observability.IncMeterDLQ()
			rdb.PushDLQ(ctx, msg.Values)
			rdb.AckMeterEvent(ctx, consumerGroup, msg.ID)
		}
		return
	}
	rdb.AckMeterEvent(ctx, consumerGroup, msg.ID)
}

func replayDLQ(cfg config.Config, log *slog.Logger) {
	ctx := context.Background()
	rdb, err := qredis.Connect(cfg.RedisURL)
	if err != nil {
		log.Error("redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()
	messages, err := rdb.ReadDLQ(ctx, 100)
	if err != nil {
		log.Error("read dlq", "err", err)
		os.Exit(1)
	}
	for _, msg := range messages {
		if err := rdb.ReplayDLQEntry(ctx, msg.Values); err != nil {
			log.Error("replay", "err", err, "id", msg.ID)
		} else {
			log.Info("replayed", "id", msg.ID)
		}
	}
}

func runSweeper(ctx context.Context, st *store.Store, rdb *qredis.Client, log *slog.Logger, holdMinutes int) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ids, err := st.GetExpiredSessions(ctx, 100)
			if err != nil {
				log.Error("sweeper list", "err", err)
				continue
			}
			for _, id := range ids {
				hold, _ := st.GetMeteringSession(ctx, id)
				u, err := st.GetUsageLogByRequestID(ctx, id)
				if err != nil {
					continue
				}
				usage := &models.UsageLog{
					Status:          "failed",
					RawUsage:        mustJSON(map[string]interface{}{"abandoned": true}),
					NormalizedUsage: mustJSON(map[string]interface{}{"micro_credits": 0}),
					LatencyMs:       nil,
				}
				_, err = st.CaptureAndRelease(ctx, u.UserID, id, 0, hold, usage, "")
				if err != nil {
					log.Error("sweeper capture", "err", err, "request_id", id)
					continue
				}
				st.DeleteMeteringSession(ctx, id)
				rdb.DeleteHold(ctx, id.String())
				log.Info("released abandoned hold", "request_id", id)
			}
		}
	}
}

func runReconciliation(ctx context.Context, st *store.Store, rdb *qredis.Client, log *slog.Logger, autoFix bool) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := st.Pool().Query(ctx, `SELECT id, credit_balance_micro FROM users`)
			if err != nil {
				log.Error("reconcile query", "err", err)
				continue
			}
			for rows.Next() {
				var id uuid.UUID
				var cached int64
				if err := rows.Scan(&id, &cached); err != nil {
					continue
				}
				ledger, err := st.LedgerBalance(ctx, id)
				if err != nil {
					continue
				}
				if ledger != cached {
					observability.IncBalanceDrift()
					log.Error("balance drift", "user_id", id, "cached", cached, "ledger", ledger, "delta", ledger-cached)
					if autoFix {
						if err := st.SetUserBalanceFromLedger(ctx, id, ledger); err != nil {
							log.Error("reconcile fix", "err", err, "user_id", id)
							continue
						}
						rdb.SetBalance(ctx, id.String(), ledger)
						log.Info("reconcile fixed", "user_id", id, "ledger", ledger)
					} else {
						rdb.SetBalance(ctx, id.String(), cached)
					}
				}
			}
			rows.Close()
		}
	}
}

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
