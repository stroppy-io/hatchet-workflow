package identity

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	// nsProviderDeployCred is F1's namespace for per-tenant terraform provider
	// deploy credentials (e.g. YC_TOKEN). Deliberately distinct from
	// nsProviderSecret, which is OIDC client secrets for SSO — an unrelated
	// concept that happens to share the word "provider". See
	// docs/superpowers/specs/2026-07-08-sp-f-execution-shapeup.md §3 F1.
	nsProviderDeployCred = "provider_deploy_cred"
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

// newAEAD builds the AES-GCM cipher shared by every sealed secret adapter in
// this file, from a single configured SecretEncryptionKey (16/24/32 bytes for
// AES-128/192/256). Returns a typed FailedPrecondition error when the key is
// missing or invalid — never silently stores plaintext.
func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) == 0 {
		return nil, derrors.FailedPrecondition("identity.secret_key_missing", "secret encryption key is not configured")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, derrors.FailedPrecondition("identity.secret_key_invalid", "secret encryption key must be 16, 24 or 32 bytes").Wrap(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, derrors.FailedPrecondition("identity.secret_key_invalid", "failed to initialise AES-GCM").Wrap(err)
	}
	return aead, nil
}

// sealValue/openValue: shared AES-GCM seal/open, used by both ProviderSecrets
// (OIDC client secrets) and ProviderDeployCreds (F1: terraform deploy creds)
// so the two adapters do not duplicate sealing logic.
func sealValue(aead cipher.AEAD, plaintext string) (string, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func openValue(aead cipher.AEAD, sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", derrors.Internal("stored secret is corrupt").Wrap(err)
	}
	ns := aead.NonceSize()
	if len(raw) < ns {
		return "", derrors.Internal("stored secret is truncated")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", derrors.Internal("failed to decrypt secret").Wrap(err)
	}
	return string(plaintext), nil
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
	aead, err := newAEAD(cfg.SecretEncryptionKey)
	if err != nil {
		return nil, err
	}
	return &ProviderSecrets{store: store, aead: aead}, nil
}

func (s *ProviderSecrets) Set(ctx context.Context, providerID, secret string) error {
	sealed, err := sealValue(s.aead, secret)
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
	return openValue(s.aead, sealed)
}

func (s *ProviderSecrets) Delete(ctx context.Context, providerID string) error {
	return s.store.Remove(ctx, nsProviderSecret, providerID)
}

// ProviderDeployCreds implements provider.Deps.EnvFn's exact shape (Get) for
// F1: it resolves the per-tenant terraform-provider deploy credentials
// (YC_TOKEN and friends) previously hardcoded as a single process-wide env
// map in internal/app/run.go. Sealed at rest (same key as ProviderSecrets,
// different namespace — nsProviderDeployCred is NOT nsProviderSecret, which
// is OIDC client secrets for SSO, an unrelated concept that happens to share
// the word "provider").
type ProviderDeployCreds struct {
	store SecretStore
	aead  cipher.AEAD
}

// NewProviderDeployCreds builds the adapter over a backing store, sealing
// values with the configured SecretEncryptionKey.
func NewProviderDeployCreds(store SecretStore, cfg Config) (*ProviderDeployCreds, error) {
	aead, err := newAEAD(cfg.SecretEncryptionKey)
	if err != nil {
		return nil, err
	}
	return &ProviderDeployCreds{store: store, aead: aead}, nil
}

func (s *ProviderDeployCreds) Set(ctx context.Context, tenantID string, env map[string]string) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal provider deploy creds: %w", err)
	}
	sealed, err := sealValue(s.aead, string(raw))
	if err != nil {
		return err
	}
	return s.store.Put(ctx, nsProviderDeployCred, tenantID, sealed)
}

// Get resolves tenantID's deploy credential env map. Its signature matches
// provider.Deps.EnvFn exactly, so internal/app/run.go assigns it directly
// (EnvFn: providerDeployCreds.Get) with no wrapper. Propagates
// derrors.ErrNotFound for a tenant with no configured credentials — the
// caller (NewProviderForRef) must surface that as an explicit provisioning
// error, never fall back to a process-wide credential set.
func (s *ProviderDeployCreds) Get(ctx context.Context, tenantID string) (map[string]string, error) {
	sealed, err := s.store.Fetch(ctx, nsProviderDeployCred, tenantID)
	if err != nil {
		return nil, err
	}
	plaintext, err := openValue(s.aead, sealed)
	if err != nil {
		return nil, err
	}
	var env map[string]string
	if err := json.Unmarshal([]byte(plaintext), &env); err != nil {
		return nil, derrors.Internal("stored provider deploy creds are corrupt").Wrap(err)
	}
	return env, nil
}

func (s *ProviderDeployCreds) Delete(ctx context.Context, tenantID string) error {
	return s.store.Remove(ctx, nsProviderDeployCred, tenantID)
}
