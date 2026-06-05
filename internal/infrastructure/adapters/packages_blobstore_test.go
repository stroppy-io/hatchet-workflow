package adapters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
