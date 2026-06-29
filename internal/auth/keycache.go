package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/models"
)

// KeyCacheMeta is the Redis-serialized auth cache entry for a QuarkGate API key.
type KeyCacheMeta struct {
	ID           string          `json:"id"`
	UserID       string          `json:"user_id"`
	KeyPrefix    string          `json:"key_prefix"`
	KeyHash      string          `json:"key_hash"`
	Name         string          `json:"name"`
	Scopes       json.RawMessage `json:"scopes"`
	RateLimitRPM int             `json:"rate_limit_rpm"`
	Status       string          `json:"status"`
}

const KeyCacheTTL = 24 * time.Hour

func KeyToCache(k *models.QuarkGateKey) (string, error) {
	meta := KeyCacheMeta{
		ID:           k.ID.String(),
		UserID:       k.UserID.String(),
		KeyPrefix:    k.KeyPrefix,
		KeyHash:      k.KeyHash,
		Name:         k.Name,
		Scopes:       k.Scopes,
		RateLimitRPM: k.RateLimitRPM,
		Status:       k.Status,
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func KeyFromCache(s string) (*models.QuarkGateKey, error) {
	var meta KeyCacheMeta
	if err := json.Unmarshal([]byte(s), &meta); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(meta.ID)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(meta.UserID)
	if err != nil {
		return nil, err
	}
	if meta.Status != "active" {
		return nil, fmt.Errorf("key not active")
	}
	return &models.QuarkGateKey{
		ID:           id,
		UserID:       userID,
		KeyPrefix:    meta.KeyPrefix,
		KeyHash:      meta.KeyHash,
		Name:         meta.Name,
		Scopes:       meta.Scopes,
		RateLimitRPM: meta.RateLimitRPM,
		Status:       meta.Status,
	}, nil
}
