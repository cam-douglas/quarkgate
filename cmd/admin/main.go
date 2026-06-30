package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/auth"
	"github.com/quarkgate/quarkgate/internal/config"
	"github.com/quarkgate/quarkgate/internal/db"
	"github.com/quarkgate/quarkgate/internal/keys"
	"github.com/quarkgate/quarkgate/internal/models"
	"github.com/quarkgate/quarkgate/internal/registry"
	qredis "github.com/quarkgate/quarkgate/internal/redis"
	"github.com/quarkgate/quarkgate/internal/store"
	"github.com/quarkgate/quarkgate/internal/vault"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool,
		"migrations/001_initial.sql",
		"migrations/002_seed_providers.sql",
		"migrations/003_usage_ledger_links.sql",
	); err != nil {
		log.Fatal(err)
	}

	st := store.New(pool)
	v, err := vault.New(cfg.VaultKEK)
	if err != nil {
		log.Fatal(err)
	}
	vs := vault.NewStore(pool, v)

	rdb, err := qredis.Connect(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	defer rdb.Close()

	switch os.Args[1] {
	case "migrate":
		fmt.Println("migrations applied")
	case "create-user":
		if len(os.Args) < 3 {
			log.Fatal("usage: admin create-user <email>")
		}
		u, err := st.CreateUser(ctx, os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("user_id=%s email=%s\n", u.ID, u.Email)
	case "deposit-credits":
		if len(os.Args) < 4 {
			log.Fatal("usage: admin deposit-credits <user_id> <credits>")
		}
		userID, err := uuid.Parse(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		credits, err := parseCredits(os.Args[3])
		if err != nil {
			log.Fatal(err)
		}
		bal, err := st.DepositCredits(ctx, userID, credits, fmt.Sprintf("deposit-%d", time.Now().UnixNano()))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("balance_micro=%d\n", bal)
	case "create-key":
		if len(os.Args) < 4 {
			log.Fatal("usage: admin create-key <user_id> <name>")
		}
		userID, err := uuid.Parse(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		full, prefix, hash, err := keys.Generate(false)
		if err != nil {
			log.Fatal(err)
		}
		scopes := json.RawMessage(`["*"]`)
		k, err := st.CreateKey(ctx, userID, os.Args[3], prefix, hash, scopes, 120)
		if err != nil {
			log.Fatal(err)
		}
		if err := cacheAPIKey(ctx, rdb, cfg.VaultKEK, full, k); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("key_id=%s api_key=%s (save this; shown once)\n", k.ID, full)
	case "revoke-key":
		if len(os.Args) < 3 {
			log.Fatal("usage: admin revoke-key <key_id>")
		}
		keyID, err := uuid.Parse(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		if err := st.RevokeKey(ctx, keyID); err != nil {
			log.Fatal(err)
		}
		if hmac, ok, _ := rdb.GetKeyIDIndex(ctx, keyID.String()); ok {
			rdb.DeleteKeyMeta(ctx, hmac)
			rdb.DeleteKeyIDIndex(ctx, keyID.String())
		}
		fmt.Println("key revoked")
	case "store-credential":
		if len(os.Args) < 5 {
			log.Fatal("usage: admin store-credential <provider_slug> <label> <secret>")
		}
		p, err := st.GetProviderBySlug(ctx, os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		if err := vs.Put(ctx, p.ID.String(), os.Args[3], os.Args[4]); err != nil {
			log.Fatal(err)
		}
		fmt.Println("credential stored")
	case "seed-provider":
		fmt.Println("providers seeded via migrations")
	case "driver-health":
		if len(os.Args) < 3 {
			log.Fatal("usage: admin driver-health <provider_slug>")
		}
		slug := os.Args[2]
		reg, err := registry.Load(cfg.DriversPath, cfg.NodePath)
		if err != nil {
			log.Fatal(err)
		}
		p, err := st.GetProviderBySlug(ctx, slug)
		if err != nil {
			log.Fatal(err)
		}
		var authInject map[string]string
		json.Unmarshal(p.AuthInjection, &authInject)
		label := authInject["vault_label"]
		if label == "" {
			label = "master_" + slug
		}
		cred, err := vs.Get(ctx, slug, label)
		if err != nil {
			log.Fatal(err)
		}
		h, err := reg.InvokeHealthCheck(slug, p.BaseURL, cred)
		if err != nil {
			log.Fatal(err)
		}
		out, _ := json.Marshal(h)
		fmt.Println(string(out))
	case "balance":
		if len(os.Args) < 3 {
			log.Fatal("usage: admin balance <user_id>")
		}
		userID, err := uuid.Parse(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		cached, ledger, err := st.ReconcileUserBalance(ctx, userID)
		if err != nil {
			log.Fatal(err)
		}
		redisBal, ok, _ := rdb.GetBalance(ctx, userID.String())
		fmt.Printf("cached_micro=%d ledger_micro=%d redis_micro=%d redis_cached=%v\n", cached, ledger, redisBal, ok)
	case "usage-summary":
		if len(os.Args) < 3 {
			log.Fatal("usage: admin usage-summary <user_id>")
		}
		userID, err := uuid.Parse(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		rows, err := st.UsageSummary24h(ctx, userID)
		if err != nil {
			log.Fatal(err)
		}
		for _, r := range rows {
			fmt.Printf("provider=%s captured_micro=%d count=%d\n", r.ProviderSlug, r.TotalMicro, r.Count)
		}
	case "burn-rate":
		if len(os.Args) < 3 {
			log.Fatal("usage: admin burn-rate <user_id>")
		}
		userID, err := uuid.Parse(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		total, err := st.BurnRate24h(ctx, userID)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("burn_24h_micro=%d burn_per_hour_micro=%d\n", total, total/24)
	case "usage-log":
		if len(os.Args) < 3 {
			log.Fatal("usage: admin usage-log <request_id>")
		}
		rid, err := uuid.Parse(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		u, err := st.GetUsageLogByRequestID(ctx, rid)
		if err != nil {
			log.Fatal(err)
		}
		out, _ := json.Marshal(u)
		fmt.Println(string(out))
		txns, err := st.ListLedgerByReference(ctx, rid)
		if err != nil {
			log.Fatal(err)
		}
		for _, t := range txns {
			fmt.Printf("ledger txn=%s type=%s amount_micro=%d\n", t.ID, t.Type, t.AmountMicro)
		}
	case "reconcile-user":
		if len(os.Args) < 3 {
			log.Fatal("usage: admin reconcile-user <user_id>")
		}
		userID, err := uuid.Parse(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		cached, ledger, err := st.ReconcileUserBalance(ctx, userID)
		if err != nil {
			log.Fatal(err)
		}
		if cached == ledger {
			fmt.Println("no drift")
			return
		}
		if err := st.SetUserBalanceFromLedger(ctx, userID, ledger); err != nil {
			log.Fatal(err)
		}
		rdb.SetBalance(ctx, userID.String(), ledger)
		fmt.Printf("fixed cached=%d -> ledger=%d\n", cached, ledger)
	case "replay-dlq":
		messages, err := rdb.ReadDLQ(ctx, 100)
		if err != nil {
			log.Fatal(err)
		}
		for _, msg := range messages {
			if err := rdb.ReplayDLQEntry(ctx, msg.Values); err != nil {
				log.Fatal(err)
			}
			fmt.Println("replayed", msg.ID)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func cacheAPIKey(ctx context.Context, rdb *qredis.Client, pepper, token string, k *models.QuarkGateKey) error {
	cached, err := auth.KeyToCache(k)
	if err != nil {
		return err
	}
	hmac := keys.HMACHash(token, pepper)
	if err := rdb.SetKeyMeta(ctx, hmac, cached); err != nil {
		return err
	}
	return rdb.SetKeyIDIndex(ctx, k.ID.String(), hmac)
}

func parseCredits(s string) (int64, error) {
	var credits int64
	_, err := fmt.Sscan(s, &credits)
	if err != nil {
		return 0, err
	}
	return credits * 1_000_000, nil
}

func usage() {
	fmt.Println("quarkgate admin commands:")
	fmt.Println("  migrate")
	fmt.Println("  create-user <email>")
	fmt.Println("  deposit-credits <user_id> <credits>")
	fmt.Println("  create-key <user_id> <name>")
	fmt.Println("  revoke-key <key_id>")
	fmt.Println("  store-credential <provider_slug> <label> <secret>")
	fmt.Println("  seed-provider")
	fmt.Println("  driver-health <provider_slug>")
	fmt.Println("  balance <user_id>")
	fmt.Println("  usage-summary <user_id>")
	fmt.Println("  burn-rate <user_id>")
	fmt.Println("  usage-log <request_id>")
	fmt.Println("  reconcile-user <user_id>")
	fmt.Println("  replay-dlq")
}
