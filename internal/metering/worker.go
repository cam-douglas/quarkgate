package metering

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/config"
	"github.com/quarkgate/quarkgate/internal/models"
	"github.com/quarkgate/quarkgate/internal/observability"
	"github.com/quarkgate/quarkgate/internal/registry"
	qredis "github.com/quarkgate/quarkgate/internal/redis"
	"github.com/quarkgate/quarkgate/internal/store"
)

type Worker struct {
	Store    *store.Store
	Redis    *qredis.Client
	Registry *registry.Registry
	Config   config.Config
	Log      *slog.Logger
}

type ProcessResult struct {
	CaptureMicro int64
	ReleaseMicro int64
}

func (w *Worker) ProcessEvent(ctx context.Context, fields map[string]interface{}) error {
	requestIDStr, _ := fields["request_id"].(string)
	userIDStr, _ := fields["user_id"].(string)
	provider, _ := fields["provider"].(string)
	status, _ := fields["status"].(string)
	holdMicro := toInt64(fields["hold_micro"])
	rawStr, _ := fields["raw_usage"].(string)
	durationMs := int(toInt64(fields["duration_ms"]))
	clientIdem, _ := fields["idempotency_key"].(string)
	envelopeStr, _ := fields["envelope"].(string)
	partial := fields["partial"] == true || fields["partial"] == "true"

	requestID, err := uuid.Parse(requestIDStr)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return err
	}

	var raw map[string]interface{}
	if rawStr != "" {
		json.Unmarshal([]byte(rawStr), &raw)
	}
	if raw == nil {
		raw = map[string]interface{}{}
	}

	var envelopeJSON json.RawMessage
	if envelopeStr != "" {
		envelopeJSON = json.RawMessage(envelopeStr)
	} else {
		op, _ := fields["operation"].(string)
		envelopeJSON, _ = json.Marshal(map[string]string{
			"provider":  provider,
			"operation": op,
		})
	}

	// Inject model from envelope when missing
	if envelopeStr != "" {
		var envelope map[string]interface{}
		if json.Unmarshal(envelopeJSON, &envelope) == nil {
			if payload, ok := envelope["payload"].(map[string]interface{}); ok {
				if model, ok := payload["model"].(string); ok && model != "" {
					if _, has := raw["model"]; !has {
						raw["model"] = model
					}
				}
			}
		}
	}

	if w.Registry != nil && w.Registry.HasCapability(provider, "normalize_usage") {
		normalized, err := w.Registry.InvokeNormalize(provider, raw, envelopeJSON)
		if err != nil {
			w.Log.Warn("driver normalize failed", "provider", provider, "err", err)
		} else {
			raw = normalized
		}
	}

	providerCfg, err := w.Store.GetProviderBySlug(ctx, provider)
	if err != nil {
		return err
	}

	margin := w.Config.PlatformMargin
	captureMicro, normJSON, err := NormalizeWithOptions(providerCfg.PricingModel, raw, w.Config.CreditUSDMicro, NormalizeOptions{
		PlatformMargin: margin,
		Partial:        partial || status == "partial",
	})
	if err != nil {
		return err
	}
	if captureMicro == 0 && status == "failed" {
		captureMicro = 0
	}

	captureMicro, releaseMicro := CaptureRelease(holdMicro, captureMicro)

	usageStatus := "completed"
	if status == "failed" {
		usageStatus = "failed"
	} else if status == "partial" || partial {
		usageStatus = "partial"
	}

	usage := &models.UsageLog{
		Status:          usageStatus,
		RawUsage:        mustJSON(raw),
		NormalizedUsage: normJSON,
		LatencyMs:       &durationMs,
	}

	txnIDs, err := w.Store.CaptureAndRelease(ctx, userID, requestID, captureMicro, releaseMicro, usage, clientIdem)
	if err != nil {
		return err
	}

	w.Store.DeleteMeteringSession(ctx, requestID)
	w.Redis.DeleteHold(ctx, requestID.String())

	u, err := w.Store.GetUser(ctx, userID)
	if err == nil {
		w.Redis.SetBalance(ctx, userID.String(), u.CreditBalanceMicro)
	}

	observability.IncMeterProcessed()
	observability.IncCreditsCaptured(captureMicro)
	w.Log.Info("meter processed",
		"request_id", requestID,
		"user_id", userID,
		"provider", provider,
		"capture_micro", captureMicro,
		"release_micro", releaseMicro,
		"capture_txn", txnIDs.CaptureTxnID,
		"release_txn", txnIDs.ReleaseTxnID,
	)
	return nil
}

func toInt64(v interface{}) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		var n int64
		json.Unmarshal([]byte(t), &n)
		return n
	}
	return 0
}

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
