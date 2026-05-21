package webhooksender

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignatureHeaderConst(t *testing.T) {
	require.Equal(t, "X-Stroppy-Signature", SignatureHeader)
}

func TestSignDeterministic(t *testing.T) {
	secret := "topsecret"
	payload := []byte(`{"event":"run.finished","id":"abc"}`)

	first := sign(secret, payload)
	second := sign(secret, payload)

	require.Equal(t, first, second, "same secret+payload must produce the same signature")
	require.NotEmpty(t, first)
}

func TestSignDifferentSecret(t *testing.T) {
	payload := []byte("identical payload")

	sigA := sign("secret-a", payload)
	sigB := sign("secret-b", payload)

	require.NotEqual(t, sigA, sigB, "different secrets must produce different signatures")
}

func TestSignDifferentPayload(t *testing.T) {
	secret := "shared"

	sigA := sign(secret, []byte("payload-a"))
	sigB := sign(secret, []byte("payload-b"))

	require.NotEqual(t, sigA, sigB, "different payloads must produce different signatures")
}

func TestSignIsValidHexHMACSHA256(t *testing.T) {
	secret := "my-webhook-secret"
	payload := []byte(`{"hello":"world"}`)

	got := sign(secret, payload)

	// Must be valid lowercase hex.
	raw, err := hex.DecodeString(got)
	require.NoError(t, err, "signature must be valid hex")
	// HMAC-SHA256 is 32 bytes => 64 hex chars.
	require.Len(t, raw, sha256.Size)
	require.Len(t, got, sha256.Size*2)

	// Recompute the HMAC independently and compare.
	mac := hmac.New(sha256.New, []byte(secret))
	_, err = mac.Write(payload)
	require.NoError(t, err)
	want := hex.EncodeToString(mac.Sum(nil))

	require.Equal(t, want, got)
	require.True(t, hmac.Equal(raw, mac.Sum(nil)), "decoded signature must match crypto/hmac output")
}

func TestSignEmptyPayload(t *testing.T) {
	got := sign("secret", nil)

	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(nil)
	want := hex.EncodeToString(mac.Sum(nil))

	require.Equal(t, want, got)
}

// TestSendSetsSignedHeaderAndReturnsOnSuccess exercises Send() against an
// httptest server: it verifies the signature header is set to sign(secret,
// payload) and that a 2xx response yields no error.
func TestSendSetsSignedHeaderAndReturnsOnSuccess(t *testing.T) {
	secret := "hook-secret"
	payload := []byte(`{"k":"v"}`)
	wantSig := sign(secret, payload)

	var (
		mu       sync.Mutex
		gotSig   string
		gotCT    string
		gotBody  []byte
		gotCalls int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotCalls++
		gotSig = r.Header.Get(SignatureHeader)
		gotCT = r.Header.Get("Content-Type")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = buf
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := New()
	err := s.Send(context.Background(), srv.URL, secret, payload)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, gotCalls)
	require.Equal(t, wantSig, gotSig)
	require.Equal(t, "application/json", gotCT)
	require.Equal(t, payload, gotBody)
}

// TestSendNoSignatureWhenSecretEmpty verifies the signature header is omitted
// when no secret is configured.
func TestSendNoSignatureWhenSecretEmpty(t *testing.T) {
	var hasHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasHeader = r.Header[SignatureHeader]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := New().Send(context.Background(), srv.URL, "", []byte("body"))
	require.NoError(t, err)
	require.False(t, hasHeader, "no signature header expected without a secret")
}

// TestSendClientErrorIsFinal verifies a 4xx response is returned as a final
// error (single attempt, no retries).
func TestSendClientErrorIsFinal(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	err := New().Send(context.Background(), srv.URL, "s", []byte("body"))
	require.Error(t, err)
	require.Equal(t, 1, calls, "4xx must not be retried")
}
