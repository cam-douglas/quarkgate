package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/quarkgate/quarkgate/internal/models"
	"github.com/quarkgate/quarkgate/internal/registry"
	qredis "github.com/quarkgate/quarkgate/internal/redis"
	"github.com/quarkgate/quarkgate/internal/store"
	"github.com/quarkgate/quarkgate/internal/vault"
	"github.com/quarkgate/quarkgate/internal/metering"
)

type RouteMiddleware struct {
	reg   *registry.Registry
	store *store.Store
	vault *vault.Store
	log   *slog.Logger
}

func NewRouteMiddleware(reg *registry.Registry, st *store.Store, vs *vault.Store, log *slog.Logger) *RouteMiddleware {
	return &RouteMiddleware{reg: reg, store: st, vault: vs, log: log}
}

func (rm *RouteMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match, err := rm.reg.MatchRoute(r.Method, r.URL.Path)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown route"})
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body"})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		var envelope *models.QuarkGateEnvelope
		if match.Operation == "envelope" {
			envelope, err = registry.ReadEnvelope(bytes.NewReader(body))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid envelope"})
				return
			}
			match.Provider = envelope.Provider
			match.Operation = envelope.Operation
		} else if match.Compat && match.Provider == "openrouter" {
			envelope = &models.QuarkGateEnvelope{
				QuarkgateVersion: "1",
				Provider:         "openrouter",
				Operation:        "chat.completions.create",
				Payload:          json.RawMessage(body),
				TraceID:          TraceID(r.Context()),
			}
		} else {
			envelope = &models.QuarkGateEnvelope{
				QuarkgateVersion: "1",
				Provider:         match.Provider,
				Operation:        match.Operation,
				Payload:          json.RawMessage(body),
				TraceID:          TraceID(r.Context()),
			}
		}

		provider, err := rm.store.GetProviderBySlug(r.Context(), match.Provider)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
			return
		}

		var vaultLabel string
		var authInject map[string]string
		json.Unmarshal(provider.AuthInjection, &authInject)
		vaultLabel = authInject["vault_label"]
		if vaultLabel == "" {
			vaultLabel = "master_" + match.Provider
		}

		credential, err := rm.vault.Get(r.Context(), match.Provider, vaultLabel)
		if err != nil {
			rm.log.Error("vault credential missing", "provider", match.Provider, "err", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credential unavailable"})
			return
		}

		envelopeBytes, _ := json.Marshal(envelope)
		downstream, err := rm.reg.InvokePrepare(match.Provider, envelopeBytes, credential, provider.BaseURL)
		if err != nil {
			rm.log.Error("driver prepare failed", "err", err)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "transform failed", "detail": err.Error()})
			return
		}

		hold := downstream.EstimateMicro
		if envelope.MeteringHints != nil && envelope.MeteringHints.EstimatedMaxCredits > 0 {
			hold = envelope.MeteringHints.EstimatedMaxCredits * 1_000_000
		}
		if hold == 0 {
			hold = metering.EstimateMax(provider.PricingModel, 0, 10_000_000)
		}

		ctx := WithRoute(r.Context(), match)
		ctx = WithEnvelope(ctx, envelope)
		ctx = WithDownstream(ctx, downstream)
		ctx = WithHoldMicro(ctx, hold)
		ctx = WithProviderCredential(ctx, credential)
		ctx = WithProviderBaseURL(ctx, provider.BaseURL)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func EmitMeterEvent(ctx context.Context, redisClient *qredis.Client, event models.MeteringEvent) {
	fields := map[string]interface{}{
		"request_id":  event.RequestID,
		"user_id":     event.UserID,
		"key_id":      event.KeyID,
		"provider":    event.Provider,
		"operation":   event.Operation,
		"status":      event.Status,
		"raw_usage":   string(event.RawUsage),
		"partial":     event.Partial,
		"duration_ms": event.DurationMs,
		"timestamp":   event.Timestamp,
		"error":       event.Error,
		"hold_micro":  event.HoldMicro,
	}
	if event.IdempotencyKey != "" {
		fields["idempotency_key"] = event.IdempotencyKey
	}
	if len(event.Envelope) > 0 {
		fields["envelope"] = string(event.Envelope)
	}
	redisClient.EmitMeterEvent(ctx, fields)
}
