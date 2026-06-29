package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	qredis "github.com/quarkgate/quarkgate/internal/redis"
)

const idempotencyTTL = 24 * time.Hour

// CachedResponse is stored in Redis for idempotent replays.
type CachedResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type IdempotencyMiddleware struct {
	redis *qredis.Client
}

func NewIdempotencyMiddleware(r *qredis.Client) *IdempotencyMiddleware {
	return &IdempotencyMiddleware{redis: r}
}

func (m *IdempotencyMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		ctx := WithIdempotencyKey(r.Context(), idemKey)
		r = r.WithContext(ctx)

		if idemKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		if isStreamingRequest(r) {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "idempotency not supported for streaming requests",
			})
			return
		}

		userID := UserID(ctx)
		if userID != uuid.Nil {
			cacheKey := idempotencyCacheKey(userID.String(), idemKey, r.URL.Path)
			if cached, ok, err := m.getCached(r.Context(), cacheKey); err == nil && ok {
				replayCached(w, cached)
				return
			}
		}

		rec := &idempotencyRecorder{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			status:         http.StatusOK,
			headers:        make(map[string]string),
		}
		next.ServeHTTP(rec, r)

		if userID != uuid.Nil && rec.status >= 200 && rec.status < 300 {
			cacheKey := idempotencyCacheKey(userID.String(), idemKey, r.URL.Path)
			entry := CachedResponse{
				Status:  rec.status,
				Headers: rec.headers,
				Body:    rec.body.String(),
			}
			b, _ := json.Marshal(entry)
			m.redis.SetIdempotency(r.Context(), cacheKey, string(b), idempotencyTTL)
		}
	})
}

func isStreamingRequest(r *http.Request) bool {
	downstream := Downstream(r.Context())
	if downstream != nil && downstream.Streaming {
		return true
	}
	envelope := Envelope(r.Context())
	if envelope != nil {
		var payload map[string]interface{}
		if json.Unmarshal(envelope.Payload, &payload) == nil {
			if stream, ok := payload["stream"].(bool); ok && stream {
				return true
			}
		}
	}
	return false
}

func (m *IdempotencyMiddleware) getCached(ctx context.Context, key string) (CachedResponse, bool, error) {
	raw, ok, err := m.redis.GetIdempotency(ctx, key)
	if err != nil || !ok {
		return CachedResponse{}, false, err
	}
	var c CachedResponse
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return CachedResponse{}, false, err
	}
	return c, true, nil
}

func replayCached(w http.ResponseWriter, c CachedResponse) {
	for k, v := range c.Headers {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		w.Header().Set(k, v)
	}
	w.WriteHeader(c.Status)
	w.Write([]byte(c.Body))
}

func idempotencyCacheKey(userID, idemKey, path string) string {
	return fmt.Sprintf("%s:%s:%s", userID, idemKey, path)
}

type idempotencyRecorder struct {
	http.ResponseWriter
	body    *bytes.Buffer
	status  int
	headers map[string]string
}

func (r *idempotencyRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *idempotencyRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *idempotencyRecorder) Header() http.Header {
	h := r.ResponseWriter.Header()
	for k, vals := range h {
		if len(vals) > 0 {
			r.headers[k] = vals[0]
		}
	}
	return h
}

// ReadAndRestoreBody reads request body and restores r.Body for downstream handlers.
func ReadAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	return b, nil
}
