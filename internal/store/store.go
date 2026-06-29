package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quarkgate/quarkgate/internal/models"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateUser(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email) VALUES ($1)
		RETURNING id, email, status, credit_balance_micro, created_at, updated_at
	`, email).Scan(&u.ID, &u.Email, &u.Status, &u.CreditBalanceMicro, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, status, credit_balance_micro, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.Status, &u.CreditBalanceMicro, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) DepositCredits(ctx context.Context, userID uuid.UUID, amountMicro int64, idempotencyKey string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var balance int64
	err = tx.QueryRow(ctx, `
		UPDATE users SET credit_balance_micro = credit_balance_micro + $2, updated_at = NOW()
		WHERE id = $1
		RETURNING credit_balance_micro
	`, userID, amountMicro).Scan(&balance)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO credit_ledger_transactions
			(user_id, type, amount_micro, balance_after_micro, reference_type, idempotency_key)
		VALUES ($1, 'deposit', $2, $3, 'admin', $4)
	`, userID, amountMicro, balance, idempotencyKey)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return balance, nil
}

func (s *Store) CreateKey(ctx context.Context, userID uuid.UUID, name, prefix, hash string, scopes json.RawMessage, rpm int) (*models.QuarkGateKey, error) {
	var k models.QuarkGateKey
	err := s.pool.QueryRow(ctx, `
		INSERT INTO quarkgate_keys (user_id, key_prefix, key_hash, name, scopes, rate_limit_rpm)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, key_prefix, key_hash, name, scopes, rate_limit_rpm, status, last_used_at, created_at, revoked_at
	`, userID, prefix, hash, name, scopes, rpm).Scan(
		&k.ID, &k.UserID, &k.KeyPrefix, &k.KeyHash, &k.Name, &k.Scopes,
		&k.RateLimitRPM, &k.Status, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt,
	)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *Store) FindKeyByHash(ctx context.Context, hash string) (*models.QuarkGateKey, error) {
	var k models.QuarkGateKey
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, key_prefix, key_hash, name, scopes, rate_limit_rpm, status, last_used_at, created_at, revoked_at
		FROM quarkgate_keys WHERE key_hash = $1 AND status = 'active'
	`, hash).Scan(
		&k.ID, &k.UserID, &k.KeyPrefix, &k.KeyHash, &k.Name, &k.Scopes,
		&k.RateLimitRPM, &k.Status, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt,
	)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *Store) ListProviderConfigs(ctx context.Context) ([]models.ProviderConfig, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, provider_slug, display_name, category, base_url, auth_injection, pricing_model,
		       health_check_path, enabled, driver_module
		FROM provider_configs WHERE enabled = true
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ProviderConfig
	for rows.Next() {
		var p models.ProviderConfig
		if err := rows.Scan(&p.ID, &p.ProviderSlug, &p.DisplayName, &p.Category, &p.BaseURL,
			&p.AuthInjection, &p.PricingModel, &p.HealthCheckPath, &p.Enabled, &p.DriverModule); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProviderBySlug(ctx context.Context, slug string) (*models.ProviderConfig, error) {
	var p models.ProviderConfig
	err := s.pool.QueryRow(ctx, `
		SELECT id, provider_slug, display_name, category, base_url, auth_injection, pricing_model,
		       health_check_path, enabled, driver_module
		FROM provider_configs WHERE provider_slug = $1 AND enabled = true
	`, slug).Scan(&p.ID, &p.ProviderSlug, &p.DisplayName, &p.Category, &p.BaseURL,
		&p.AuthInjection, &p.PricingModel, &p.HealthCheckPath, &p.Enabled, &p.DriverModule)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) CreateUsageLog(ctx context.Context, log *models.UsageLog) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO usage_logs
			(id, user_id, quarkgate_key_id, provider_slug, operation, request_id, status,
			 credits_reserved_micro, trace_id, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, log.ID, log.UserID, log.QuarkGateKeyID, log.ProviderSlug, log.Operation,
		log.RequestID, log.Status, log.CreditsReservedMicro, log.TraceID, log.StartedAt)
	return err
}

func (s *Store) CreateMeteringSession(ctx context.Context, requestID, userID uuid.UUID, holdMicro int64, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO metering_sessions (request_id, user_id, hold_micro, expires_at)
		VALUES ($1, $2, $3, $4)
	`, requestID, userID, holdMicro, expiresAt)
	return err
}

func (s *Store) DeleteMeteringSession(ctx context.Context, requestID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM metering_sessions WHERE request_id = $1`, requestID)
	return err
}

func (s *Store) GetExpiredSessions(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT request_id FROM metering_sessions WHERE expires_at < NOW() LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) ApplyHold(ctx context.Context, userID uuid.UUID, holdMicro int64, requestID uuid.UUID, idempotencyKey string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var balance int64
	err = tx.QueryRow(ctx, `
		SELECT credit_balance_micro FROM users WHERE id = $1 FOR UPDATE
	`, userID).Scan(&balance)
	if err != nil {
		return 0, err
	}
	if balance < holdMicro {
		return balance, fmt.Errorf("insufficient credits")
	}

	newBalance := balance - holdMicro
	_, err = tx.Exec(ctx, `
		UPDATE users SET credit_balance_micro = $2, updated_at = NOW() WHERE id = $1
	`, userID, newBalance)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO credit_ledger_transactions
			(user_id, type, amount_micro, balance_after_micro, reference_type, reference_id, idempotency_key)
		VALUES ($1, 'hold', $2, $3, 'usage_log', $4, $5)
	`, userID, -holdMicro, newBalance, requestID, idempotencyKey)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newBalance, nil
}

type LedgerTxnIDs struct {
	CaptureTxnID *uuid.UUID
	ReleaseTxnID *uuid.UUID
}

func (s *Store) CaptureAndRelease(ctx context.Context, userID, requestID uuid.UUID, captureMicro, releaseMicro int64, usage *models.UsageLog, clientIdem string) (*LedgerTxnIDs, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	captureIdem := fmt.Sprintf("capture-%s", requestID)
	releaseIdem := fmt.Sprintf("release-%s", requestID)
	if clientIdem != "" {
		captureIdem = fmt.Sprintf("capture-%s", clientIdem)
		releaseIdem = fmt.Sprintf("release-%s", clientIdem)
	}

	var balance int64
	err = tx.QueryRow(ctx, `SELECT credit_balance_micro FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&balance)
	if err != nil {
		return nil, err
	}

	out := &LedgerTxnIDs{}
	if captureMicro > 0 {
		balance -= captureMicro
		var captureID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO credit_ledger_transactions
				(user_id, type, amount_micro, balance_after_micro, reference_type, reference_id, idempotency_key)
			VALUES ($1, 'capture', $2, $3, 'usage_log', $4, $5)
			RETURNING id
		`, userID, -captureMicro, balance, requestID, captureIdem).Scan(&captureID)
		if err != nil {
			return nil, err
		}
		out.CaptureTxnID = &captureID
	}

	if releaseMicro > 0 {
		balance += releaseMicro
		var releaseID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO credit_ledger_transactions
				(user_id, type, amount_micro, balance_after_micro, reference_type, reference_id, idempotency_key)
			VALUES ($1, 'release', $2, $3, 'usage_log', $4, $5)
			RETURNING id
		`, userID, releaseMicro, balance, requestID, releaseIdem).Scan(&releaseID)
		if err != nil {
			return nil, err
		}
		out.ReleaseTxnID = &releaseID
	}

	_, err = tx.Exec(ctx, `UPDATE users SET credit_balance_micro = $2, updated_at = NOW() WHERE id = $1`, userID, balance)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE usage_logs SET
			status = $2, raw_usage = $3, normalized_usage = $4,
			credits_captured_micro = $5, latency_ms = $6, completed_at = NOW(),
			capture_txn_id = $7, release_txn_id = $8
		WHERE request_id = $1
	`, requestID, usage.Status, usage.RawUsage, usage.NormalizedUsage, captureMicro, usage.LatencyMs,
		out.CaptureTxnID, out.ReleaseTxnID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) LedgerBalance(ctx context.Context, userID uuid.UUID) (int64, error) {
	var sum int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_micro), 0) FROM credit_ledger_transactions WHERE user_id = $1
	`, userID).Scan(&sum)
	return sum, err
}

func (s *Store) GetUsageLogByRequestID(ctx context.Context, requestID uuid.UUID) (*models.UsageLog, error) {
	var u models.UsageLog
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, quarkgate_key_id, provider_slug, operation, request_id, status,
		       raw_usage, normalized_usage, credits_reserved_micro, credits_captured_micro,
		       latency_ms, trace_id, started_at, completed_at
		FROM usage_logs WHERE request_id = $1
	`, requestID).Scan(
		&u.ID, &u.UserID, &u.QuarkGateKeyID, &u.ProviderSlug, &u.Operation, &u.RequestID,
		&u.Status, &u.RawUsage, &u.NormalizedUsage, &u.CreditsReservedMicro, &u.CreditsCapturedMicro,
		&u.LatencyMs, &u.TraceID, &u.StartedAt, &u.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetMeteringSession(ctx context.Context, requestID uuid.UUID) (holdMicro int64, err error) {
	err = s.pool.QueryRow(ctx, `SELECT hold_micro FROM metering_sessions WHERE request_id = $1`, requestID).Scan(&holdMicro)
	return holdMicro, err
}

func (s *Store) IsIdempotencyUsed(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM credit_ledger_transactions WHERE idempotency_key = $1)
	`, key).Scan(&exists)
	return exists, err
}

func (s *Store) RevokeKey(ctx context.Context, keyID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE quarkgate_keys SET status = 'revoked', revoked_at = NOW()
		WHERE id = $1 AND status = 'active'
	`, keyID)
	return err
}

func (s *Store) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *Store) ReconcileUserBalance(ctx context.Context, userID uuid.UUID) (cached, ledger int64, err error) {
	u, err := s.GetUser(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	ledger, err = s.LedgerBalance(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	return u.CreditBalanceMicro, ledger, nil
}

func (s *Store) SetUserBalanceFromLedger(ctx context.Context, userID uuid.UUID, ledgerSum int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET credit_balance_micro = $2, updated_at = NOW() WHERE id = $1
	`, userID, ledgerSum)
	return err
}

type UsageSummaryRow struct {
	ProviderSlug string
	TotalMicro   int64
	Count        int64
}

func (s *Store) UsageSummary24h(ctx context.Context, userID uuid.UUID) ([]UsageSummaryRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT provider_slug, COALESCE(SUM(credits_captured_micro), 0), COUNT(*)
		FROM usage_logs
		WHERE user_id = $1 AND started_at > NOW() - INTERVAL '24 hours'
		GROUP BY provider_slug
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageSummaryRow
	for rows.Next() {
		var r UsageSummaryRow
		if err := rows.Scan(&r.ProviderSlug, &r.TotalMicro, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) BurnRate24h(ctx context.Context, userID uuid.UUID) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(credits_captured_micro), 0)
		FROM usage_logs
		WHERE user_id = $1 AND started_at > NOW() - INTERVAL '24 hours'
	`, userID).Scan(&total)
	return total, err
}

func (s *Store) ListLedgerByReference(ctx context.Context, requestID uuid.UUID) ([]models.LedgerTransaction, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, type, amount_micro, balance_after_micro, reference_type, reference_id, idempotency_key, created_at
		FROM credit_ledger_transactions
		WHERE reference_id = $1
		ORDER BY created_at
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.LedgerTransaction
	for rows.Next() {
		var t models.LedgerTransaction
		if err := rows.Scan(&t.ID, &t.UserID, &t.Type, &t.AmountMicro, &t.BalanceAfterMicro,
			&t.ReferenceType, &t.ReferenceID, &t.IdempotencyKey, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
