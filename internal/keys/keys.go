package keys

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	LivePrefix = "qg_live_"
	TestPrefix = "qg_test_"
)

func Generate(test bool) (fullKey, prefix, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(b)
	if test {
		fullKey = TestPrefix + secret
	} else {
		fullKey = LivePrefix + secret
	}
	prefix = fullKey[:12]
	hash, err = Hash(fullKey)
	return fullKey, prefix, hash, err
}

func Hash(key string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func Compare(hash, key string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key)) == nil
}

// HMACHash is a fast cache key for Redis lookups.
func HMACHash(key, pepper string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(key))
	return hex.EncodeToString(mac.Sum(nil))
}

func ValidateFormat(key string) error {
	if !hasPrefix(key, LivePrefix) && !hasPrefix(key, TestPrefix) {
		return fmt.Errorf("invalid key format")
	}
	if len(key) < 20 {
		return fmt.Errorf("key too short")
	}
	return nil
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
