-- QuarkGate initial schema

CREATE TYPE user_status AS ENUM ('active', 'suspended', 'closed');
CREATE TYPE key_status AS ENUM ('active', 'revoked');
CREATE TYPE provider_category AS ENUM ('llm', 'scraper', 'memory', 'execution', 'ui');
CREATE TYPE ledger_txn_type AS ENUM ('deposit', 'hold', 'capture', 'release', 'adjustment', 'refund');
CREATE TYPE usage_log_status AS ENUM ('pending', 'completed', 'failed', 'partial');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    status user_status NOT NULL DEFAULT 'active',
    credit_balance_micro BIGINT NOT NULL DEFAULT 0 CHECK (credit_balance_micro >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE quarkgate_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    rate_limit_rpm INT NOT NULL DEFAULT 60,
    status key_status NOT NULL DEFAULT 'active',
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_quarkgate_keys_user_id ON quarkgate_keys(user_id);

CREATE TABLE provider_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_slug TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    category provider_category NOT NULL,
    base_url TEXT NOT NULL,
    auth_injection JSONB NOT NULL DEFAULT '{}'::jsonb,
    pricing_model JSONB NOT NULL DEFAULT '{}'::jsonb,
    health_check_path TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    driver_module TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE credential_vault_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_config_id UUID NOT NULL REFERENCES provider_configs(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    encrypted_blob BYTEA NOT NULL,
    key_version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at TIMESTAMPTZ,
    UNIQUE (provider_config_id, label)
);

CREATE TABLE credit_ledger_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type ledger_txn_type NOT NULL,
    amount_micro BIGINT NOT NULL,
    balance_after_micro BIGINT NOT NULL,
    reference_type TEXT,
    reference_id UUID,
    idempotency_key TEXT UNIQUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledger_user_created ON credit_ledger_transactions(user_id, created_at DESC);

CREATE TABLE usage_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    quarkgate_key_id UUID NOT NULL REFERENCES quarkgate_keys(id) ON DELETE CASCADE,
    provider_slug TEXT NOT NULL,
    operation TEXT NOT NULL,
    request_id UUID NOT NULL UNIQUE,
    status usage_log_status NOT NULL DEFAULT 'pending',
    raw_usage JSONB NOT NULL DEFAULT '{}'::jsonb,
    normalized_usage JSONB NOT NULL DEFAULT '{}'::jsonb,
    credits_reserved_micro BIGINT NOT NULL DEFAULT 0,
    credits_captured_micro BIGINT NOT NULL DEFAULT 0,
    latency_ms INT,
    trace_id TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_usage_logs_user_started ON usage_logs(user_id, started_at DESC);

CREATE TABLE metering_sessions (
    request_id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hold_micro BIGINT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    stream_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_metering_sessions_expires ON metering_sessions(expires_at);
