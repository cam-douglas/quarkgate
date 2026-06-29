//go:build integration

package metering_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/config"
	"github.com/quarkgate/quarkgate/internal/db"
	"github.com/quarkgate/quarkgate/internal/metering"
	"github.com/quarkgate/quarkgate/internal/models"
	"github.com/quarkgate/quarkgate/internal/registry"
	qredis "github.com/quarkgate/quarkgate/internal/redis"
	"github.com/quarkgate/quarkgate/internal/store"
)

func TestMeteringLoop(t *testing.T) {
	ctx := context.Background()
	cfg := config.Load()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skip("postgres unavailable")
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool,
		"migrations/001_initial.sql",
		"migrations/002_seed_providers.sql",
		"migrations/003_usage_ledger_links.sql",
	); err != nil {
		t.Fatal(err)
	}

	rdb, err := qredis.Connect(cfg.RedisURL)
	if err != nil {
		t.Skip("redis unavailable")
	}
	defer rdb.Close()

	st := store.New(pool)
	u, err := st.CreateUser(ctx, "meter-test-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	bal, err := st.DepositCredits(ctx, u.ID, 50_000_000, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	requestID := uuid.New()
	hold := 5_000_000
	_, err = st.ApplyHold(ctx, u.ID, hold, requestID, "")
	if err != nil {
		t.Fatal(err)
	}

	keyID := uuid.New()
	log := &models.UsageLog{
		ID:                   uuid.New(),
		UserID:               u.ID,
		QuarkGateKeyID:       keyID,
		ProviderSlug:         "openrouter",
		Operation:            "chat.completions.create",
		RequestID:            requestID,
		Status:               "pending",
		CreditsReservedMicro: hold,
		StartedAt:            time.Now(),
	}
	if err := st.CreateUsageLog(ctx, log); err != nil {
		t.Fatal(err)
	}
	st.CreateMeteringSession(ctx, requestID, u.ID, hold, time.Now().Add(15*time.Minute))

	reg, _ := registry.Load(cfg.DriversPath, cfg.NodePath)
	worker := &metering.Worker{
		Store:    st,
		Redis:    rdb,
		Registry: reg,
		Config:   cfg,
		Log:      nil,
	}

	raw, _ := json.Marshal(map[string]interface{}{
		"input_tokens":  10,
		"output_tokens": 20,
	})
	fields := map[string]interface{}{
		"request_id":  requestID.String(),
		"user_id":     u.ID.String(),
		"provider":    "openrouter",
		"operation":   "chat.completions.create",
		"status":      "completed",
		"raw_usage":   string(raw),
		"hold_micro":  hold,
		"duration_ms": 100,
	}
	if err := worker.ProcessEvent(ctx, fields); err != nil {
		t.Fatal(err)
	}

	u2, err := st.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u2.CreditBalanceMicro >= bal {
		t.Fatalf("expected balance drop from %d got %d", bal, u2.CreditBalanceMicro)
	}

	usage, err := st.GetUsageLogByRequestID(ctx, requestID)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Status != "completed" {
		t.Fatalf("status %s", usage.Status)
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("INTEGRATION") != "1" {
		os.Exit(0)
	}
	m.Run()
}
