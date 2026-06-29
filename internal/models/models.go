package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                 uuid.UUID
	Email              string
	Status             string
	CreditBalanceMicro int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type QuarkGateKey struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	KeyPrefix    string
	KeyHash      string
	Name         string
	Scopes       json.RawMessage
	RateLimitRPM int
	Status       string
	LastUsedAt   *time.Time
	CreatedAt    time.Time
	RevokedAt    *time.Time
}

type ProviderConfig struct {
	ID              uuid.UUID
	ProviderSlug    string
	DisplayName     string
	Category        string
	BaseURL         string
	AuthInjection   json.RawMessage
	PricingModel    json.RawMessage
	HealthCheckPath string
	Enabled         bool
	DriverModule    string
}

type UsageLog struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	QuarkGateKeyID        uuid.UUID
	ProviderSlug          string
	Operation             string
	RequestID             uuid.UUID
	Status                string
	RawUsage              json.RawMessage
	NormalizedUsage       json.RawMessage
	CreditsReservedMicro  int64
	CreditsCapturedMicro  int64
	LatencyMs             *int
	TraceID               string
	StartedAt             time.Time
	CompletedAt           *time.Time
	CaptureTxnID          *uuid.UUID
	ReleaseTxnID          *uuid.UUID
}

type LedgerTransaction struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Type              string
	AmountMicro       int64
	BalanceAfterMicro int64
	ReferenceType     string
	ReferenceID       *uuid.UUID
	IdempotencyKey    string
	CreatedAt         time.Time
}

type MeteringEvent struct {
	RequestID  string          `json:"request_id"`
	UserID     string          `json:"user_id"`
	KeyID      string          `json:"key_id,omitempty"`
	Provider   string          `json:"provider"`
	Operation  string          `json:"operation"`
	Status     string          `json:"status"`
	RawUsage   json.RawMessage `json:"raw_usage,omitempty"`
	Partial    bool            `json:"partial"`
	DurationMs int             `json:"duration_ms"`
	Timestamp  string          `json:"timestamp"`
	Error      string          `json:"error,omitempty"`
	HoldMicro      int64           `json:"hold_micro"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Envelope       json.RawMessage `json:"envelope,omitempty"`
}

type QuarkGateEnvelope struct {
	QuarkgateVersion string          `json:"quarkgate_version"`
	Provider         string          `json:"provider"`
	Operation        string          `json:"operation"`
	Payload          json.RawMessage `json:"payload"`
	MeteringHints    *MeteringHints  `json:"metering_hints,omitempty"`
	TraceID          string          `json:"trace_id,omitempty"`
}

type MeteringHints struct {
	EstimatedMaxCredits int64  `json:"estimated_max_credits,omitempty"`
	Priority            string `json:"priority,omitempty"`
}

type DownstreamRequest struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers"`
	Body       json.RawMessage   `json:"body,omitempty"`
	Streaming  bool              `json:"streaming"`
	Provider   string            `json:"provider"`
	Operation  string            `json:"operation"`
	EstimateMicro int64           `json:"estimate_micro"`
}

type RouteMatch struct {
	Provider  string
	Operation string
	Compat    bool
}
