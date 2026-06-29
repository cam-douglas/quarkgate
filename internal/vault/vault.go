package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Vault provides AES-256-GCM envelope encryption for credentials.
type Vault struct {
	gcm cipher.AEAD
}

func New(kek string) (*Vault, error) {
	key := deriveKey(kek)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("vault cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("vault gcm: %w", err)
	}
	return &Vault{gcm: gcm}, nil
}

func deriveKey(kek string) []byte {
	key := make([]byte, 32)
	copy(key, []byte(kek))
	return key
}

// Encrypt returns nonce+ciphertext blob.
func (v *Vault) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, v.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return v.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt unwraps nonce+ciphertext blob.
func (v *Vault) Decrypt(blob []byte) ([]byte, error) {
	nonceSize := v.gcm.NonceSize()
	if len(blob) < nonceSize {
		return nil, errors.New("vault: blob too short")
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	return v.gcm.Open(nil, nonce, ciphertext, nil)
}

// EncryptString encrypts and returns base64.
func (v *Vault) EncryptString(s string) (string, error) {
	b, err := v.Encrypt([]byte(s))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecryptString decrypts base64 blob.
func (v *Vault) DecryptString(encoded string) (string, error) {
	blob, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	plain, err := v.Decrypt(blob)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
