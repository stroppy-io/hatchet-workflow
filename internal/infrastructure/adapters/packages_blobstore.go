// Package adapters implements the non-storage / non-iam / non-temporal port
// interfaces declared by the connect services (internal/services/*). Each
// adapter is constructor-injected and satisfies exactly one service port; the
// storage repos (gormstore), execution/temporal adapters and iam/auth adapters
// live in their own packages and are wired in alongside these.
package adapters

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
)

type AgentTokenVerifier interface {
	VerifyAgentToken(token string) (*agentdomain.TokenClaims, error)
}

// LocalBlobStore is a local-filesystem implementation of packages.BlobStore. It
// roots every package blob under a configured directory keyed by the
// server-assigned storage key. The presigned PUT "url" it mints is a direct
// gateway path (relative URL) that the server's package-upload route serves;
// there is no external object store, so the signature is the storage key plus a
// short-lived expiry plus declared blob metadata.
type LocalBlobStore struct {
	// root is the directory blobs are stored under (one file per storage key).
	root string
	// uploadPathPrefix is the gateway route prefix the presigned PUT url points
	// at (e.g. "/api/packages/upload"). The minted url is
	// {prefix}/{escaped-key}.
	uploadPathPrefix string
	// uploadSecret signs local presigned PUT URLs. It is process-local; pending
	// URLs are deliberately invalidated by a control-plane restart.
	uploadSecret []byte
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
	uploadPathPrefix = strings.TrimRight(uploadPathPrefix, "/")
	if uploadPathPrefix == "" {
		uploadPathPrefix = "/api/packages/upload"
	}
	secret := make([]byte, 32)
	if _, err := cryptorand.Read(secret); err != nil {
		return nil, fmt.Errorf("packages: generate upload signer: %w", err)
	}
	return &LocalBlobStore{
		root:             root,
		uploadPathPrefix: uploadPathPrefix,
		uploadSecret:     secret,
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
// declared size/sha256 on receipt and binds them into the signed URL.
func (s *LocalBlobStore) PresignPut(_ context.Context, key string, expectedSize uint64, sha string, ttl time.Duration) (string, time.Time, error) {
	if strings.TrimSpace(key) == "" {
		return "", time.Time{}, errors.New("packages: storage key is required")
	}
	if expectedSize == 0 || expectedSize > math.MaxInt64 {
		return "", time.Time{}, errors.New("packages: expected size is invalid")
	}
	sha, err := normalizeSHA256Hex(sha)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(ttl)
	expiresUnix := expiresAt.Unix()
	q := url.Values{}
	q.Set("expires", strconv.FormatInt(expiresUnix, 10))
	q.Set("size", strconv.FormatUint(expectedSize, 10))
	q.Set("sha256", sha)
	q.Set("token", s.uploadToken(key, expectedSize, sha, expiresUnix))
	u := s.uploadPathPrefix + "/" + url.PathEscape(key) + "?" + q.Encode()
	return u, expiresAt, nil
}

// UploadPathPrefix is the route prefix served by UploadHandler.
func (s *LocalBlobStore) UploadPathPrefix() string { return s.uploadPathPrefix }

func (s *LocalBlobStore) DownloadPathPrefix() string { return "/api/packages/blob" }

// UploadHandler accepts PUTs to presigned local upload URLs minted by PresignPut.
func (s *LocalBlobStore) UploadHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.Header().Set("Allow", http.MethodPut)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		key, expectedSize, expectedSha, err := s.verifyUploadRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if r.ContentLength >= 0 && uint64(r.ContentLength) != expectedSize {
			http.Error(w, "content length does not match signed size", http.StatusBadRequest)
			return
		}
		if err := s.storeUpload(w, r, key, expectedSize, expectedSha); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func (s *LocalBlobStore) DownloadHandler(verifier AgentTokenVerifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		keyPart := strings.TrimPrefix(r.URL.Path, s.DownloadPathPrefix()+"/")
		if keyPart == "" || keyPart == r.URL.Path {
			http.Error(w, "invalid package blob path", http.StatusBadRequest)
			return
		}
		key, err := url.PathUnescape(keyPart)
		if err != nil || strings.TrimSpace(key) == "" {
			http.Error(w, "invalid package blob key", http.StatusBadRequest)
			return
		}
		if verifier != nil {
			claims, ok := agentClaimsFromBearer(r.Header.Get("Authorization"), verifier)
			if !ok {
				http.Error(w, "invalid agent token", http.StatusUnauthorized)
				return
			}
			if !strings.HasPrefix(key, "packages_"+claims.TenantID+"_") {
				http.Error(w, "package blob tenant mismatch", http.StatusForbidden)
				return
			}
		}
		path := s.filePath(key)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "package blob unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, path)
	})
}

func (s *LocalBlobStore) verifyUploadRequest(r *http.Request) (string, uint64, string, error) {
	keyPart := strings.TrimPrefix(r.URL.Path, s.uploadPathPrefix+"/")
	if keyPart == "" || keyPart == r.URL.Path {
		return "", 0, "", errors.New("invalid upload path")
	}
	key, err := url.PathUnescape(keyPart)
	if err != nil || strings.TrimSpace(key) == "" {
		return "", 0, "", errors.New("invalid upload key")
	}
	q := r.URL.Query()
	expiresUnix, err := strconv.ParseInt(q.Get("expires"), 10, 64)
	if err != nil || expiresUnix <= time.Now().Unix() {
		return "", 0, "", errors.New("upload URL expired")
	}
	expectedSize, err := strconv.ParseUint(q.Get("size"), 10, 64)
	if err != nil || expectedSize == 0 || expectedSize > math.MaxInt64 {
		return "", 0, "", errors.New("invalid signed size")
	}
	expectedSha, err := normalizeSHA256Hex(q.Get("sha256"))
	if err != nil {
		return "", 0, "", errors.New("invalid signed sha256")
	}
	wantToken := s.uploadToken(key, expectedSize, expectedSha, expiresUnix)
	gotToken := q.Get("token")
	if !hmac.Equal([]byte(gotToken), []byte(wantToken)) {
		return "", 0, "", errors.New("invalid upload token")
	}
	return key, expectedSize, expectedSha, nil
}

func agentClaimsFromBearer(header string, verifier AgentTokenVerifier) (*agentdomain.TokenClaims, bool) {
	if verifier == nil {
		return nil, false
	}
	token := agentdomain.BearerToken(header)
	if token == "" {
		return nil, false
	}
	claims, err := verifier.VerifyAgentToken(token)
	if err != nil {
		return nil, false
	}
	return claims, true
}

func (s *LocalBlobStore) storeUpload(w http.ResponseWriter, r *http.Request, key string, expectedSize uint64, expectedSha string) error {
	tmp, err := os.CreateTemp(s.root, ".package-upload-*")
	if err != nil {
		return fmt.Errorf("packages: create temp upload: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	h := sha256.New()
	limited := http.MaxBytesReader(w, r.Body, int64(expectedSize)+1) //nolint:gosec // checked against MaxInt64 above.
	written, copyErr := io.Copy(io.MultiWriter(tmp, h), limited)
	closeErr := tmp.Close()
	if copyErr != nil {
		return fmt.Errorf("packages: write upload: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("packages: close upload: %w", closeErr)
	}
	if uint64(written) != expectedSize { //nolint:gosec // written is non-negative for nil copyErr.
		return fmt.Errorf("uploaded size mismatch: expected %d bytes, got %d", expectedSize, written)
	}
	actualSha := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actualSha, expectedSha) {
		return fmt.Errorf("uploaded sha256 mismatch: expected %s, got %s", expectedSha, actualSha)
	}
	if err := os.Rename(tmpName, s.filePath(key)); err != nil {
		return fmt.Errorf("packages: commit upload: %w", err)
	}
	return nil
}

func (s *LocalBlobStore) uploadToken(key string, expectedSize uint64, sha string, expiresUnix int64) string {
	mac := hmac.New(sha256.New, s.uploadSecret)
	_, _ = fmt.Fprintf(mac, "%s\n%d\n%s\n%d", key, expectedSize, sha, expiresUnix)
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizeSHA256Hex(sha string) (string, error) {
	sha = strings.ToLower(strings.TrimSpace(sha))
	if len(sha) != sha256.Size*2 {
		return "", errors.New("packages: sha256 is invalid")
	}
	if _, err := hex.DecodeString(sha); err != nil {
		return "", errors.New("packages: sha256 is invalid")
	}
	return sha, nil
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
