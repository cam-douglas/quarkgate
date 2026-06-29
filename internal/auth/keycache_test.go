package auth

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/models"
)

func TestKeyCacheRoundTrip(t *testing.T) {
	orig := &models.QuarkGateKey{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		KeyPrefix:    "qg_live_ab",
		KeyHash:      "hash",
		Name:         "test",
		Scopes:       json.RawMessage(`["*"]`),
		RateLimitRPM: 60,
		Status:       "active",
	}
	s, err := KeyToCache(orig)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := KeyFromCache(s)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != orig.ID || restored.UserID != orig.UserID {
		t.Fatal("id mismatch")
	}
	if restored.RateLimitRPM != 60 {
		t.Fatal("rpm mismatch")
	}
}

func TestKeyFromCacheRejectsRevoked(t *testing.T) {
	s, _ := json.Marshal(KeyCacheMeta{Status: "revoked"})
	if _, err := KeyFromCache(string(s)); err == nil {
		t.Fatal("expected error for revoked")
	}
}
