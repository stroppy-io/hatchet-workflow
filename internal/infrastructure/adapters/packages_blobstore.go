// Package adapters implements the non-storage / non-iam / non-temporal port
// interfaces declared by the connect services (internal/services/*). Each
// adapter is constructor-injected and satisfies exactly one service port; the
// storage repos (gormstore), execution/temporal adapters and iam/auth adapters
// live in their own packages and are wired in alongside these.
package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
)

// LocalBlobStore is a local-filesystem implementation of packages.BlobStore. It
// roots every package blob under a configured directory keyed by the
// server-assigned storage key. The presigned PUT "url" it mints is a direct
// gateway path (relative URL) that the server's package-upload route serves;
// there is no external object store, so the signature is the storage key plus a
// short-lived expiry the caller enforces.
type LocalBlobStore struct {
	// root is the directory blobs are stored under (one file per storage key).
	root string
	// uploadPathPrefix is the gateway route prefix the presigned PUT url points
	// at (e.g. "/api/packages/upload"). The minted url is
	// {prefix}/{escaped-key}.
	uploadPathPrefix string
}

var _ packages.BlobStore = (*LocalBlobStore)(nil)

// NewLocalBlobStore builds a filesystem-backed blob store rooted at root. The
// directory is created if missing. uploadPathPrefix is the gateway route the
// minted presigned PUT url targets; an empty prefix defaults to
// "/api/packages/upload".
func NewLocalBlobStore(root, uploadPathPrefix string) (*LocalBlobStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("packages: blob store root dir is required")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("packages: create blob store root: %w", err)
	}
	if strings.TrimSpace(uploadPathPrefix) == "" {
		uploadPathPrefix = "/api/packages/upload"
	}
	return &LocalBlobStore{
		root:             root,
		uploadPathPrefix: strings.TrimRight(uploadPathPrefix, "/"),
	}, nil
}

// filePath maps a storage key to an on-disk path under root. The key is sanitized
// (base name only, no traversal) so a key can never escape the root.
func (s *LocalBlobStore) filePath(key string) string {
	clean := filepath.Base(filepath.Clean("/" + key))
	return filepath.Join(s.root, clean)
}

// PresignPut returns a direct gateway path the client PUTs the blob to plus its
// expiry. There is no external object store; the gateway route validates the
// declared size/sha256 on receipt, so they are not folded into a signature here.
func (s *LocalBlobStore) PresignPut(_ context.Context, key string, _ uint64, _ string, ttl time.Duration) (string, time.Time, error) {
	if strings.TrimSpace(key) == "" {
		return "", time.Time{}, errors.New("packages: storage key is required")
	}
	expiresAt := time.Now().Add(ttl)
	u := s.uploadPathPrefix + "/" + url.PathEscape(key)
	return u, expiresAt, nil
}

// Stat reports the uploaded blob's size and its SHA-256 content hash, recomputed
// from disk so CompleteUpload verifies what actually landed. derrors.ErrNotFound
// when the client never PUT the blob.
func (s *LocalBlobStore) Stat(_ context.Context, key string) (uint64, string, error) {
	path := s.filePath(key)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, "", derrors.NotFound("blob", "uploaded object not found")
		}
		return 0, "", fmt.Errorf("packages: stat blob: %w", err)
	}
	if info.IsDir() {
		return 0, "", derrors.NotFound("blob", "uploaded object not found")
	}
	f, err := os.Open(path) //nolint:gosec // path is sanitized to root via filePath.
	if err != nil {
		return 0, "", fmt.Errorf("packages: open blob: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return 0, "", fmt.Errorf("packages: hash blob: %w", err)
	}
	return uint64(info.Size()), hex.EncodeToString(h.Sum(nil)), nil //nolint:gosec // file size is non-negative.
}

// Delete removes the blob; idempotent (a missing blob is not an error).
func (s *LocalBlobStore) Delete(_ context.Context, key string) error {
	if err := os.Remove(s.filePath(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("packages: delete blob: %w", err)
	}
	return nil
}

// TenantPrefixedStorageKeys mints package storage keys as
// "packages/{tenantID}/{packageID}", giving each tenant its own prefix.
type TenantPrefixedStorageKeys struct{}

var _ packages.StorageKeys = TenantPrefixedStorageKeys{}

// NewStorageKeys returns the default tenant-prefixed key minter.
func NewStorageKeys() TenantPrefixedStorageKeys { return TenantPrefixedStorageKeys{} }

// PackageKey returns the object-storage key for a tenant's package blob.
func (TenantPrefixedStorageKeys) PackageKey(tenantID, packageID string) string {
	return fmt.Sprintf("packages_%s_%s", tenantID, packageID)
}

// StaticLimits is a packages.Limits backed by a fixed configured ceiling.
type StaticLimits struct{ maxPackageSizeBytes uint64 }

var _ packages.Limits = StaticLimits{}

// NewStaticLimits builds a Limits with the given max package size in bytes. A
// zero value means "no limit" (the service treats <=0 as unbounded).
func NewStaticLimits(maxPackageSizeBytes uint64) StaticLimits {
	return StaticLimits{maxPackageSizeBytes: maxPackageSizeBytes}
}

// MaxPackageSizeBytes returns the configured size ceiling.
func (l StaticLimits) MaxPackageSizeBytes() uint64 { return l.maxPackageSizeBytes }

// StaticUploadTTL is a packages.UploadTTL backed by a fixed configured lifetime.
type StaticUploadTTL struct{ ttl time.Duration }

var _ packages.UploadTTL = StaticUploadTTL{}

// NewStaticUploadTTL builds an UploadTTL with the given presigned-url lifetime. A
// non-positive ttl defaults to 15 minutes.
func NewStaticUploadTTL(ttl time.Duration) StaticUploadTTL {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return StaticUploadTTL{ttl: ttl}
}

// UploadURLTTL returns the configured presigned PUT url lifetime.
func (t StaticUploadTTL) UploadURLTTL() time.Duration { return t.ttl }
