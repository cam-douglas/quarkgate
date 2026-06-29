package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/gateway"
	"github.com/quarkgate/quarkgate/internal/models"
)

func TestMeteringReaderParsesUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		w.Write([]byte("data: {\"usage\":{\"input_tokens\":10,\"output_tokens\":20}}\n\n"))
		flusher.Flush()
		w.Write([]byte("data: [DONE]\n\n"))
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
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestCircuitBreakerOpens(t *testing.T) {
	cb := NewCircuitBreaker(0.5, time.Minute)
	for i := 0; i < 5; i++ {
		cb.Record(false)
	}
	if cb.Allow("test") {
		t.Fatal("expected circuit open")
	}
}

func TestParseJSONUsage(t *testing.T) {
	raw := map[string]interface{}{
		"usage": map[string]interface{}{
			"input_tokens":  5,
			"output_tokens": 10,
		},
	}
	b, _ := json.Marshal(raw)
	u := parseJSONUsage(b)
	if u == nil {
		t.Fatal("nil usage")
	}
}

func TestSSEParserLine(t *testing.T) {
	mr := &meteringReader{
		w: io.Discard,
	}
	mr.parseLine([]byte("data: {\"usage\":{\"output_tokens\":42}}\n"))
	if mr.usage == nil {
		t.Fatal("usage not parsed")
	}
}

func TestHandlerNoDownstream(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h := NewHandler(time.Second, 30, 1800, nil, nil, nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", rec.Code)
	}
}

func TestMeteringReaderCopy(t *testing.T) {
	src := bytes.NewBufferString("data: {\"x\":1}\n\n")
	rec := &bytes.Buffer{}
	mr := &meteringReader{src: src, w: rec, flusher: nil}
	_, err := mr.copy()
	if err != nil {
		t.Fatal(err)
	}
	if rec.Len() == 0 {
		t.Fatal("empty output")
	}
}
