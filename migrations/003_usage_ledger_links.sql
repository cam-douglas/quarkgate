-- Link usage_logs to ledger capture/release transactions for audit

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS capture_txn_id UUID REFERENCES credit_ledger_transactions(id),
    ADD COLUMN IF NOT EXISTS release_txn_id UUID REFERENCES credit_ledger_transactions(id);
