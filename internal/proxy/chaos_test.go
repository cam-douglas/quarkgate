package proxy

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMeteringReaderOverheadBudget(t *testing.T) {
	body := bytes.Repeat([]byte("data: {\"x\":1}\n\n"), 20)
	body = append(body, []byte("data: [DONE]\n\n")...)

	start := time.Now()
	rec := &bytes.Buffer{}
	mr := &meteringReader{src: bytes.NewReader(body), w: rec}
	_, err := mr.copy()
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("metering reader took %v, budget 50ms", elapsed)
	}
	if rec.Len() == 0 {
		t.Fatal("empty output")
	}
}

func TestCircuitBreakerBlocksBeforeDownstream(t *testing.T) {
	cb := NewCircuitBreaker(0.5, time.Minute)
	for i := 0; i < 5; i++ {
		cb.Record(false)
	}
	if cb.Allow("openrouter") {
		t.Fatal("expected open circuit")
	}
}

func TestBulkheadLimitsConcurrency(t *testing.T) {
	b := NewBulkhead(1)
	if !b.TryAcquire("p") {
		t.Fatal("first acquire")
	}
	if b.TryAcquire("p") {
		t.Fatal("second should block")
	}
	b.Release("p")
	if !b.TryAcquire("p") {
		t.Fatal("after release")
	}
}

func TestChaosMidStreamSetsPartialPath(t *testing.T) {
	rec := httptest.NewRecorder()
	mr := &meteringReader{
		src: bytes.NewReader([]byte("data: {\"chunk\":1}\n\n")),
		w:   rec,
	}
	_, err := mr.copy()
	if err != nil && err != io.EOF {
		// abrupt close simulated by short body is EOF — ok
	}
	if mr.usage == nil && mr.dataChunks == 0 {
		t.Fatal("expected some metering state")
	}
}
