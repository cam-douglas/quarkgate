package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/gateway"
	"github.com/quarkgate/quarkgate/internal/models"
)

// P2.7: byte-identical SSE passthrough and usage token extraction.
func TestIntegrationSSEPassthroughAndUsage(t *testing.T) {
	expectedBody := "data: {\"usage\":{\"input_tokens\":10,\"output_tokens\":20}}\n\ndata: [DONE]\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		w.Write([]byte(expectedBody))
		flusher.Flush()
	}))
	defer srv.Close()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := gateway.WithRequestID(req.Context(), uuid.New())
	ctx = gateway.WithUserID(ctx, uuid.New())
	ctx = gateway.WithKeyID(ctx, uuid.New())
	ctx = gateway.WithRoute(ctx, &models.RouteMatch{Provider: "openrouter", Operation: "chat.completions.create"})
	ctx = gateway.WithHoldMicro(ctx, 1_000_000)
	ctx = gateway.WithDownstream(ctx, &models.DownstreamRequest{
		URL:        srv.URL,
		Method:     http.MethodPost,
		Headers:    map[string]string{},
		Streaming:  true,
		Provider:   "openrouter",
		Operation:  "chat.completions.create",
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h := NewHandler(5*time.Second, 30, 1800, nil, nil, nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Body.String() != expectedBody {
		t.Fatalf("body mismatch:\ngot:  %q\nwant: %q", rec.Body.String(), expectedBody)
	}
}

func TestIntegrationMeteringReaderUsageTokens(t *testing.T) {
	stream := "data: {\"usage\":{\"input_tokens\":10,\"output_tokens\":20}}\n\ndata: [DONE]\n\n"
	rec := &strings.Builder{}
	mr := newMeteringReader(context.Background(), strings.NewReader(stream), rec, nil, 30, 0, 0)
	if _, err := mr.copy(); err != nil {
		t.Fatal(err)
	}
	if rec.String() != stream {
		t.Fatalf("passthrough mismatch")
	}
	if mr.usage == nil {
		t.Fatal("usage nil")
	}
	in, _ := mr.usage["input_tokens"].(float64)
	out, _ := mr.usage["output_tokens"].(float64)
	if int(in) != 10 || int(out) != 20 {
		t.Fatalf("usage tokens got in=%v out=%v", in, out)
	}
}

func TestIntegrationFallbackChunkEstimate(t *testing.T) {
	stream := "data: {\"chunk\":1}\n\ndata: {\"chunk\":2}\n\n"
	rec := &strings.Builder{}
	mr := newMeteringReader(context.Background(), strings.NewReader(stream), rec, nil, 30, 0, 0)
	if _, err := mr.copy(); err != nil {
		t.Fatal(err)
	}
	if mr.usage == nil {
		t.Fatal("expected fallback usage")
	}
	if mr.usage["fallback"] != "chunk_estimate" {
		t.Fatalf("expected fallback estimate")
	}
}
