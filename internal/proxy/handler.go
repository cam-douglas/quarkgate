package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/quarkgate/quarkgate/internal/gateway"
	"github.com/quarkgate/quarkgate/internal/models"
	"github.com/quarkgate/quarkgate/internal/observability"
	"github.com/quarkgate/quarkgate/internal/registry"
	qredis "github.com/quarkgate/quarkgate/internal/redis"
)

type Handler struct {
	client       *http.Client
	streamClient *http.Client
	redis        *qredis.Client
	reg          *registry.Registry
	log          *slog.Logger
	idleSec      int
	maxStreamSec int
	breaker      *CircuitBreaker
	bulkhead     *Bulkhead
}

func NewHandler(connectTimeout time.Duration, idleSec, maxStreamSec int, redis *qredis.Client, reg *registry.Registry, log *slog.Logger) *Handler {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	return &Handler{
		client:       &http.Client{Timeout: connectTimeout, Transport: transport},
		streamClient: &http.Client{Transport: transport},
		redis:        redis,
		reg:          reg,
		log:          log,
		idleSec:      idleSec,
		maxStreamSec: maxStreamSec,
	}
}

func (h *Handler) SetBreaker(b *CircuitBreaker) {
	h.breaker = b
}

func (h *Handler) SetBulkhead(b *Bulkhead) {
	h.bulkhead = b
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	IncActive()
	defer DecActive()

	observability.IncRequests()
	start := time.Now()
	downstream := gateway.Downstream(r.Context())
	if downstream == nil {
		http.Error(w, "no downstream", http.StatusInternalServerError)
		return
	}

	if h.bulkhead != nil && !h.bulkhead.TryAcquire(downstream.Provider) {
		w.Header().Set("Retry-After", "5")
		writeErr(w, http.StatusServiceUnavailable, "bulkhead full")
		h.emitMeter(r.Context(), r, start, "failed", false, nil)
		return
	}
	if h.bulkhead != nil {
		defer h.bulkhead.Release(downstream.Provider)
	}

	if h.breaker != nil && !h.breaker.Allow(downstream.Provider) {
		w.Header().Set("Retry-After", "30")
		writeErr(w, http.StatusServiceUnavailable, "circuit open")
		h.emitMeter(r.Context(), r, start, "failed", false, nil)
		return
	}

	bodyReader := io.NopCloser(bytes.NewReader(downstream.Body))
	if len(downstream.Body) == 0 && r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		bodyReader = io.NopCloser(bytes.NewReader(b))
	}

	reqCtx := r.Context()
	if downstream.Streaming && h.maxStreamSec > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(reqCtx, time.Duration(h.maxStreamSec)*time.Second)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, downstream.Method, downstream.URL, bodyReader)
	if err != nil {
		h.fail(w, r, start, "build request", err)
		return
	}
	for k, v := range downstream.Headers {
		req.Header.Set(k, v)
	}

	client := h.client
	if downstream.Streaming {
		client = h.streamClient
		req.Header.Set("Accept", "text/event-stream")
	}

	resp, err := client.Do(req)
	if err != nil {
		if h.breaker != nil {
			h.breaker.Record(false)
		}
		h.fail(w, r, start, "downstream connect", err)
		return
	}
	defer resp.Body.Close()
	if h.breaker != nil {
		h.breaker.Record(resp.StatusCode < 500)
	}

	copyHeaders(w, resp)
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)

	var rawUsage map[string]interface{}
	var responseBody string
	status := "completed"
	partial := false

	if downstream.Streaming && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if rc := http.NewResponseController(w); rc != nil {
			rc.SetWriteDeadline(time.Time{})
		}
		flusher, _ := w.(http.Flusher)
		maxDur := time.Duration(h.maxStreamSec) * time.Second
		holdMicro := gateway.HoldMicro(r.Context())
		mr := newMeteringReader(r.Context(), resp.Body, w, flusher, h.idleSec, maxDur, holdMicro)
		_, parseErr := mr.copy()
		rawUsage = mr.usage
		responseBody = mr.body.String()
		if mr.softCapExceeded {
			status = "partial"
			partial = true
			writeSSEError(w, flusher, "insufficient_credits")
			if rawUsage == nil {
				rawUsage = map[string]interface{}{"soft_cap": true}
			}
		} else if parseErr != nil {
			status = "partial"
			partial = true
			writeSSEError(w, flusher, parseErr.Error())
			if rawUsage == nil {
				rawUsage = map[string]interface{}{"error": parseErr.Error()}
			}
		}
	} else {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			status = "failed"
		} else {
			w.Write(data)
			responseBody = string(data)
			rawUsage = parseJSONUsage(data)
		}
		if resp.StatusCode >= 400 {
			status = "failed"
		}
	}

	rawUsage = h.mergeDriverUsage(r.Context(), downstream.Provider, resp.Header, responseBody, downstream.Streaming, rawUsage)
	h.emitMeter(r.Context(), r, start, status, partial, rawUsage)
}

func (h *Handler) mergeDriverUsage(ctx context.Context, provider string, hdr http.Header, body string, streaming bool, base map[string]interface{}) map[string]interface{} {
	if h.reg == nil || !h.reg.HasCapability(provider, "parse_response") {
		return base
	}
	headers := map[string]string{}
	for k, vals := range hdr {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	envelope := gateway.Envelope(ctx)
	var envelopeBytes json.RawMessage
	if envelope != nil {
		envelopeBytes, _ = json.Marshal(envelope)
	}
	driverUsage, err := h.reg.InvokeParseResponse(provider, headers, body, streaming, envelopeBytes)
	if err != nil {
		h.log.Warn("driver parseResponse failed", "provider", provider, "err", err)
		return base
	}
	if driverUsage == nil {
		return base
	}
	merged := mergeMaps(base, driverUsage)
	if needsPoll(merged) && h.reg.HasCapability(provider, "async_poll") {
		credential := gateway.ProviderCredential(ctx)
		baseURL := gateway.ProviderBaseURL(ctx)
		runID, _ := merged["run_id"].(string)
		if runID != "" && credential != "" && baseURL != "" {
			pollCtx := map[string]interface{}{"run_id": runID}
			pollResult, err := h.reg.InvokePoll(provider, baseURL, credential, pollCtx)
			if err != nil {
				h.log.Warn("driver poll failed", "provider", provider, "err", err)
			} else if pollResult != nil && pollResult.RawUsage != nil {
				merged = mergeMaps(merged, pollResult.RawUsage)
				delete(merged, "needs_poll")
			}
		}
	}
	return merged
}

func needsPoll(raw map[string]interface{}) bool {
	if raw == nil {
		return false
	}
	v, ok := raw["needs_poll"]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	}
	return false
}

func mergeMaps(base, patch map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, start time.Time, msg string, err error) {
	h.log.Error(msg, "err", err)
	h.emitMeter(r.Context(), r, start, "failed", false, nil)
	w.Header().Set("Retry-After", "10")
	writeErr(w, http.StatusBadGateway, "downstream error")
}

func writeSSEError(w http.ResponseWriter, flusher http.Flusher, msg string) {
	payload, _ := json.Marshal(map[string]string{"error": msg})
	w.Write([]byte("data: " + string(payload) + "\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
}

func (h *Handler) emitMeter(ctx context.Context, r *http.Request, start time.Time, status string, partial bool, raw map[string]interface{}) {
	if h.redis == nil {
		return
	}
	route := gateway.Route(ctx)
	if raw == nil {
		raw = map[string]interface{}{}
	}
	rawBytes, _ := json.Marshal(raw)
	event := models.MeteringEvent{
		RequestID:      gateway.RequestID(ctx).String(),
		UserID:         gateway.UserID(ctx).String(),
		KeyID:          gateway.KeyID(ctx).String(),
		Provider:       route.Provider,
		Operation:      route.Operation,
		Status:         status,
		RawUsage:       rawBytes,
		Partial:        partial,
		DurationMs:     int(time.Since(start).Milliseconds()),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		HoldMicro:      gateway.HoldMicro(ctx),
		IdempotencyKey: gateway.IdempotencyKey(ctx),
	}
	if envelope := gateway.Envelope(ctx); envelope != nil {
		event.Envelope, _ = json.Marshal(envelope)
	}
	gateway.EmitMeterEvent(ctx, h.redis, event)
}

func copyHeaders(w http.ResponseWriter, resp *http.Response) {
	for k, vals := range resp.Header {
		if strings.EqualFold(k, "Connection") || strings.EqualFold(k, "Transfer-Encoding") {
			continue
		}
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
