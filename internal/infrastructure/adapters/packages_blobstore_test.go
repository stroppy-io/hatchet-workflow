package adapters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
)

func TestLocalBlobStoreUploadHandlerStoresSignedBlob(t *testing.T) {
	store, err := NewLocalBlobStore(t.TempDir(), "/api/packages/upload")
	if err != nil {
		t.Fatalf("new blob store: %v", err)
	}
	content := []byte("package bytes")
	sha := sha256Hex(content)
	uploadURL, _, err := store.PresignPut(context.Background(), "packages_tenant_pkg", uint64(len(content)), sha, time.Minute)
	if err != nil {
		t.Fatalf("presign put: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(content))
	store.UploadHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	size, gotSha, err := store.Stat(context.Background(), "packages_tenant_pkg")
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if size != uint64(len(content)) {
		t.Fatalf("size = %d, want %d", size, len(content))
	}
	if gotSha != sha {
		t.Fatalf("sha = %s, want %s", gotSha, sha)
	}
}

func TestLocalBlobStoreUploadHandlerRejectsHashMismatch(t *testing.T) {
	store, err := NewLocalBlobStore(t.TempDir(), "/api/packages/upload")
	if err != nil {
		t.Fatalf("new blob store: %v", err)
	}
	content := []byte("package bytes")
	otherSha := sha256Hex([]byte("different bytes"))
	uploadURL, _, err := store.PresignPut(context.Background(), "packages_tenant_pkg", uint64(len(content)), otherSha, time.Minute)
	if err != nil {
		t.Fatalf("presign put: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(content))
	store.UploadHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestLocalBlobStoreDownloadHandlerRequiresAgentTenant(t *testing.T) {
	store, err := NewLocalBlobStore(t.TempDir(), "/api/packages/upload")
	if err != nil {
		t.Fatalf("new blob store: %v", err)
	}
	key := "packages_tenant-1_pkg"
	content := []byte("package bytes")
	uploadPackageBlob(t, store, key, content)

	req := httptest.NewRequest(http.MethodGet, store.DownloadPathPrefix()+"/"+url.PathEscape(key), nil)
	req.Header.Set("Authorization", "Bearer good")
	rr := httptest.NewRecorder()
	store.DownloadHandler(fakeAgentVerifier{tokens: map[string]string{"good": "tenant-1"}}).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := rr.Body.Bytes(); !bytes.Equal(got, content) {
		t.Fatalf("body = %q, want %q", got, content)
	}
}

func TestLocalBlobStoreDownloadHandlerRejectsWrongTenant(t *testing.T) {
	store, err := NewLocalBlobStore(t.TempDir(), "/api/packages/upload")
	if err != nil {
		t.Fatalf("new blob store: %v", err)
	}
	key := "packages_tenant-1_pkg"
	uploadPackageBlob(t, store, key, []byte("package bytes"))

	req := httptest.NewRequest(http.MethodGet, store.DownloadPathPrefix()+"/"+url.PathEscape(key), nil)
	req.Header.Set("Authorization", "Bearer bad")
	rr := httptest.NewRecorder()
	store.DownloadHandler(fakeAgentVerifier{tokens: map[string]string{"bad": "tenant-2"}}).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func uploadPackageBlob(t *testing.T, store *LocalBlobStore, key string, content []byte) {
	t.Helper()
	uploadURL, _, err := store.PresignPut(context.Background(), key, uint64(len(content)), sha256Hex(content), time.Minute)
	if err != nil {
		t.Fatalf("presign put: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(content))
	store.UploadHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("upload status = %d, want %d: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}
}

type fakeAgentVerifier struct {
	tokens map[string]string
}

func (v fakeAgentVerifier) VerifyAgentToken(token string) (*agentdomain.TokenClaims, error) {
	tenantID, ok := v.tokens[token]
	if !ok {
		return nil, errors.New("invalid token")
	}
	return &agentdomain.TokenClaims{TenantID: tenantID}, nil
}
