package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/keys"
	"github.com/quarkgate/quarkgate/internal/models"
	"github.com/quarkgate/quarkgate/internal/auth"
	"github.com/quarkgate/quarkgate/internal/observability"
	"github.com/quarkgate/quarkgate/internal/store"
	qredis "github.com/quarkgate/quarkgate/internal/redis"
)

type AuthMiddleware struct {
	store  *store.Store
	redis  *qredis.Client
	log    *slog.Logger
	pepper string
}

func NewAuthMiddleware(st *store.Store, r *qredis.Client, log *slog.Logger, pepper string) *AuthMiddleware {
	return &AuthMiddleware{store: st, redis: r, log: log, pepper: pepper}
}

func (a *AuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearer(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}
		if err := keys.ValidateFormat(token); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token format"})
			return
		}

		k, err := a.findKeyByToken(r.Context(), token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}

		ctx := WithUserID(r.Context(), k.UserID)
		ctx = WithKeyID(ctx, k.ID)
		ctx = WithKey(ctx, k)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *AuthMiddleware) findKeyByToken(ctx context.Context, token string) (*models.QuarkGateKey, error) {
	hmac := keys.HMACHash(token, a.pepper)
	if meta, ok, err := a.redis.GetKeyMeta(ctx, hmac); err == nil && ok {
		k, err := auth.KeyFromCache(meta)
		if err == nil && keys.Compare(k.KeyHash, token) {
			return k, nil
		}
	}
	k, err := a.findKeyByTokenPG(ctx, token)
	if err != nil {
		return nil, err
	}
	if cached, err := auth.KeyToCache(k); err == nil {
		a.redis.SetKeyMeta(ctx, hmac, cached)
		a.redis.SetKeyIDIndex(ctx, k.ID.String(), hmac)
	}
	return k, nil
}

func (a *AuthMiddleware) findKeyByTokenPG(ctx context.Context, token string) (*models.QuarkGateKey, error) {
	prefix := token[:12]
	rows, err := a.store.Pool().Query(ctx, `
		SELECT id, user_id, key_prefix, key_hash, name, scopes, rate_limit_rpm, status, last_used_at, created_at, revoked_at
		FROM quarkgate_keys WHERE key_prefix = $1 AND status = 'active'
	`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k models.QuarkGateKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyPrefix, &k.KeyHash, &k.Name, &k.Scopes,
			&k.RateLimitRPM, &k.Status, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		if keys.Compare(k.KeyHash, token) {
			return &k, nil
		}
	}
	return nil, fmt.Errorf("key not found")
}

type ScopeMiddleware struct{}

func (s *ScopeMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := Key(r.Context())
		route := Route(r.Context())
		if k == nil || route == nil {
			next.ServeHTTP(w, r)
			return
		}
		var scopes []string
		if err := json.Unmarshal(k.Scopes, &scopes); err == nil {
			for _, sc := range scopes {
				if sc == "*" || sc == route.Provider || sc == route.Provider+":"+route.Operation {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "scope denied"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

type RateLimitMiddleware struct {
	redis *qredis.Client
}

func NewRateLimitMiddleware(r *qredis.Client) *RateLimitMiddleware {
	return &RateLimitMiddleware{redis: r}
}

func (rl *RateLimitMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := Key(r.Context())
		if k == nil {
			next.ServeHTTP(w, r)
			return
		}
		ok, err := rl.redis.AllowRate(r.Context(), k.UserID.String(), k.RateLimitRPM)
		if err != nil || !ok {
			w.Header().Set("Retry-After", "60")
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

type RequestIDMiddleware struct{}

func (m *RequestIDMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New()
		trace := r.Header.Get("X-Trace-Id")
		if trace == "" {
			trace = id.String()
		}
		ctx := WithRequestID(r.Context(), id)
		ctx = WithTraceID(ctx, trace)
		w.Header().Set("X-Request-Id", id.String())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

type SolvencyMiddleware struct {
	store       *store.Store
	redis       *qredis.Client
	holdTTL     time.Duration
	defaultHold int64
}

func NewSolvencyMiddleware(st *store.Store, r *qredis.Client, holdMinutes int, defaultHold int64) *SolvencyMiddleware {
	return &SolvencyMiddleware{
		store:       st,
		redis:       r,
		holdTTL:     time.Duration(holdMinutes) * time.Minute,
		defaultHold: defaultHold,
	}
}

func (s *SolvencyMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hold := HoldMicro(r.Context())
		if hold == 0 {
			hold = s.defaultHold
		}
		userID := UserID(r.Context())
		requestID := RequestID(r.Context())
		keyID := KeyID(r.Context())

		balance, cached, err := s.redis.GetBalance(r.Context(), userID.String())
		if !cached {
			u, err := s.store.GetUser(r.Context(), userID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "balance check failed"})
				return
			}
			balance = u.CreditBalanceMicro
			s.redis.SetBalance(r.Context(), userID.String(), balance)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "balance check failed"})
			return
		}
		if balance < hold {
			observability.IncHoldFailures()
			writeJSON(w, http.StatusPaymentRequired, map[string]string{
				"error":         "insufficient_credits",
				"balance_micro": fmtInt(balance),
				"required_micro": fmtInt(hold),
			})
			return
		}

	idemKey := fmt.Sprintf("hold-%s", requestID)
		if clientIdem := IdempotencyKey(r.Context()); clientIdem != "" {
			idemKey = fmt.Sprintf("hold-%s", clientIdem)
		}
		newBal, err := s.store.ApplyHold(r.Context(), userID, hold, requestID, idemKey)
		if err != nil {
			writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": "insufficient_credits"})
			return
		}
		s.redis.SetBalance(r.Context(), userID.String(), newBal)
		s.redis.SetHold(r.Context(), requestID.String(), hold, s.holdTTL)

		route := Route(r.Context())
		log := &models.UsageLog{
			ID:                   uuid.New(),
			UserID:               userID,
			QuarkGateKeyID:       keyID,
			ProviderSlug:         route.Provider,
			Operation:            route.Operation,
			RequestID:            requestID,
			Status:               "pending",
			CreditsReservedMicro: hold,
			TraceID:              TraceID(r.Context()),
			StartedAt:            time.Now(),
		}
		if err := s.store.CreateUsageLog(r.Context(), log); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "usage log failed"})
			return
		}
		expires := time.Now().Add(s.holdTTL)
		if err := s.store.CreateMeteringSession(r.Context(), requestID, userID, hold, expires); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "metering session failed"})
			return
		}

		ctx := WithHoldMicro(r.Context(), hold)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func fmtInt(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
