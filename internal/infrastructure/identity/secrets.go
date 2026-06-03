package identity

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// SecretStore is the minimal backing store the secret adapters need: an opaque
// blob keyed by a string. A namespace separates api-token hashes from provider
// secrets so the two adapters can share one store without key collisions. The
// integration layer supplies an implementation (a gorm-backed secrets table);
// Fetch returns derrors.ErrNotFound for an unknown key.
type SecretStore interface {
	Put(ctx context.Context, namespace, key, value string) error
	Fetch(ctx context.Context, namespace, key string) (string, error)
	Remove(ctx context.Context, namespace, key string) error
}

const (
	nsApiTokenHash   = "api_token_hash"
	nsProviderSecret = "provider_secret"
)

// ApiTokenSecrets implements iamsvc.ApiTokenSecrets. It persists the (already
// one-way SHA-256) token hash keyed by token id; nothing here is reversible, so
// no encryption key is required.
type ApiTokenSecrets struct {
	store SecretStore
}

var _ iamsvc.ApiTokenSecrets = (*ApiTokenSecrets)(nil)

// NewApiTokenSecrets builds the adapter over a backing secret store.
func NewApiTokenSecrets(store SecretStore) *ApiTokenSecrets {
	return &ApiTokenSecrets{store: store}
}

func (s *ApiTokenSecrets) SetHash(ctx context.Context, tokenID, hash string) error {
	return s.store.Put(ctx, nsApiTokenHash, tokenID, hash)
}

func (s *ApiTokenSecrets) GetHash(ctx context.Context, tokenID string) (string, error) {
	return s.store.Fetch(ctx, nsApiTokenHash, tokenID)
}

func (s *ApiTokenSecrets) Delete(ctx context.Context, tokenID string) error {
	return s.store.Remove(ctx, nsApiTokenHash, tokenID)
}

// ProviderSecrets implements iamsvc.ProviderSecrets. OIDC client secrets are
// recoverable (needed for the token exchange), so they are sealed with AES-GCM
// under the configured key before storage and unsealed on read.
type ProviderSecrets struct {
	store SecretStore
	aead  cipher.AEAD
}

var _ iamsvc.ProviderSecrets = (*ProviderSecrets)(nil)

// NewProviderSecrets builds the adapter over a backing store, sealing values
// with the configured SecretEncryptionKey (16/24/32 bytes for AES-128/192/256).
// Returns a typed FailedPrecondition error when the key is missing or invalid —
// never silently stores plaintext.
func NewProviderSecrets(store SecretStore, cfg Config) (*ProviderSecrets, error) {
	if len(cfg.SecretEncryptionKey) == 0 {
		return nil, derrors.FailedPrecondition("identity.secret_key_missing", "secret encryption key is not configured")
	}
	block, err := aes.NewCipher(cfg.SecretEncryptionKey)
	if err != nil {
		return nil, derrors.FailedPrecondition("identity.secret_key_invalid", "secret encryption key must be 16, 24 or 32 bytes").Wrap(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, derrors.FailedPrecondition("identity.secret_key_invalid", "failed to initialise AES-GCM").Wrap(err)
	}
	return &ProviderSecrets{store: store, aead: aead}, nil
}

func (s *ProviderSecrets) Set(ctx context.Context, providerID, secret string) error {
	sealed, err := s.seal(secret)
	if err != nil {
		return err
	}
	return s.store.Put(ctx, nsProviderSecret, providerID, sealed)
}

func (s *ProviderSecrets) Get(ctx context.Context, providerID string) (string, error) {
	sealed, err := s.store.Fetch(ctx, nsProviderSecret, providerID)
	if err != nil {
		return "", err
	}
	return s.open(sealed)
}

func (s *ProviderSecrets) Delete(ctx context.Context, providerID string) error {
	return s.store.Remove(ctx, nsProviderSecret, providerID)
}

// seal encrypts plaintext with AES-GCM and returns base64(nonce||ciphertext).
func (s *ProviderSecrets) seal(plaintext string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := s.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// open reverses seal.
func (s *ProviderSecrets) open(sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", derrors.Internal("stored provider secret is corrupt").Wrap(err)
	}
	ns := s.aead.NonceSize()
	if len(raw) < ns {
		return "", derrors.Internal("stored provider secret is truncated")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plaintext, err := s.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", derrors.Internal("failed to decrypt provider secret").Wrap(err)
	}
	return string(plaintext), nil
}
