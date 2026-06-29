package vault

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists encrypted credential in vault table.
type Store struct {
	pool  *pgxpool.Pool
	vault *Vault
}

func NewStore(pool *pgxpool.Pool, v *Vault) *Store {
	return &Store{pool: pool, vault: v}
}

func (s *Store) Put(ctx context.Context, providerConfigID, label string, plaintext string) error {
	blob, err := s.vault.Encrypt([]byte(plaintext))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO credential_vault_entries (provider_config_id, label, encrypted_blob)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider_config_id, label) DO UPDATE SET
			encrypted_blob = EXCLUDED.encrypted_blob,
			rotated_at = NOW()
	`, providerConfigID, label, blob)
	return err
}

func (s *Store) Get(ctx context.Context, providerSlug, label string) (string, error) {
	var blob []byte
	err := s.pool.QueryRow(ctx, `
		SELECT cve.encrypted_blob
		FROM credential_vault_entries cve
		JOIN provider_configs pc ON pc.id = cve.provider_config_id
		WHERE pc.provider_slug = $1 AND cve.label = $2
	`, providerSlug, label).Scan(&blob)
	if err != nil {
		return "", fmt.Errorf("vault get: %w", err)
	}
	plain, err := s.vault.Decrypt(blob)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
