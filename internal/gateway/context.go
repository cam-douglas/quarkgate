package gateway

import (
	"context"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/models"
)

type contextKey string

const (
	ctxRequestID contextKey = "request_id"
	ctxUserID    contextKey = "user_id"
	ctxKeyID     contextKey = "key_id"
	ctxKey       contextKey = "key"
	ctxRoute     contextKey = "route"
	ctxHoldMicro contextKey = "hold_micro"
	ctxTraceID   contextKey = "trace_id"
	ctxEnvelope  contextKey = "envelope"
	ctxDownstream contextKey = "downstream"
	ctxStartTime contextKey = "start_time"
	ctxIdempotencyKey     contextKey = "idempotency_key"
	ctxProviderCredential contextKey = "provider_credential"
	ctxProviderBaseURL    contextKey = "provider_base_url"
)

func WithRequestID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxRequestID, id)
}

func RequestID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(ctxRequestID).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxUserID, id)
}

func UserID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(ctxUserID).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

func WithKeyID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKeyID, id)
}

func KeyID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(ctxKeyID).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

func WithKey(ctx context.Context, k *models.QuarkGateKey) context.Context {
	return context.WithValue(ctx, ctxKey, k)
}

func Key(ctx context.Context) *models.QuarkGateKey {
	if v, ok := ctx.Value(ctxKey).(*models.QuarkGateKey); ok {
		return v
	}
	return nil
}

func WithRoute(ctx context.Context, r *models.RouteMatch) context.Context {
	return context.WithValue(ctx, ctxRoute, r)
}

func Route(ctx context.Context) *models.RouteMatch {
	if v, ok := ctx.Value(ctxRoute).(*models.RouteMatch); ok {
		return v
	}
	return nil
}

func WithHoldMicro(ctx context.Context, m int64) context.Context {
	return context.WithValue(ctx, ctxHoldMicro, m)
}

func HoldMicro(ctx context.Context) int64 {
	if v, ok := ctx.Value(ctxHoldMicro).(int64); ok {
		return v
	}
	return 0
}

func WithTraceID(ctx context.Context, t string) context.Context {
	return context.WithValue(ctx, ctxTraceID, t)
}

func TraceID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxTraceID).(string); ok {
		return v
	}
	return ""
}

func WithEnvelope(ctx context.Context, e *models.QuarkGateEnvelope) context.Context {
	return context.WithValue(ctx, ctxEnvelope, e)
}

func Envelope(ctx context.Context) *models.QuarkGateEnvelope {
	if v, ok := ctx.Value(ctxEnvelope).(*models.QuarkGateEnvelope); ok {
		return v
	}
	return nil
}

func WithDownstream(ctx context.Context, d *models.DownstreamRequest) context.Context {
	return context.WithValue(ctx, ctxDownstream, d)
}

func Downstream(ctx context.Context) *models.DownstreamRequest {
	if v, ok := ctx.Value(ctxDownstream).(*models.DownstreamRequest); ok {
		return v
	}
	return nil
}

func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, ctxIdempotencyKey, key)
}

func IdempotencyKey(ctx context.Context) string {
	if v, ok := ctx.Value(ctxIdempotencyKey).(string); ok {
		return v
	}
	return ""
}

func WithProviderCredential(ctx context.Context, credential string) context.Context {
	return context.WithValue(ctx, ctxProviderCredential, credential)
}

func ProviderCredential(ctx context.Context) string {
	if v, ok := ctx.Value(ctxProviderCredential).(string); ok {
		return v
	}
	return ""
}

func WithProviderBaseURL(ctx context.Context, baseURL string) context.Context {
	return context.WithValue(ctx, ctxProviderBaseURL, baseURL)
}

func ProviderBaseURL(ctx context.Context) string {
	if v, ok := ctx.Value(ctxProviderBaseURL).(string); ok {
		return v
	}
	return ""
}
