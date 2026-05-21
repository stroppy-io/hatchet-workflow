package auth

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateOpaqueToken_NonEmptyURLSafe(t *testing.T) {
	tok, err := GenerateOpaqueToken()
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	// Must be valid raw-url-base64 (URL-safe, no padding) and carry the full entropy.
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	require.NoError(t, err)
	require.Len(t, raw, opaqueTokenBytes)

	// URL-safe alphabet: no '+', '/', or '=' padding.
	require.NotContains(t, tok, "+")
	require.NotContains(t, tok, "/")
	require.NotContains(t, tok, "=")
}

func TestGenerateOpaqueToken_Unique(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		tok, err := GenerateOpaqueToken()
		require.NoError(t, err)
		_, dup := seen[tok]
		require.False(t, dup, "GenerateOpaqueToken produced a duplicate")
		seen[tok] = struct{}{}
	}
	require.Len(t, seen, n)
}

func TestHashToken_Deterministic(t *testing.T) {
	const tok = "some-opaque-token-value"

	h1 := HashToken(tok)
	h2 := HashToken(tok)

	// Same input -> same hash.
	require.Equal(t, h1, h2)

	// sha256 hex is 64 chars and never the plaintext.
	require.Len(t, h1, 64)
	require.NotEqual(t, tok, h1)
}

func TestHashToken_DifferentInputsDiffer(t *testing.T) {
	tests := []struct {
		name string
		a, b string
	}{
		{name: "distinct values", a: "token-a", b: "token-b"},
		{name: "one-char difference", a: "token", b: "tokeN"},
		{name: "empty vs non-empty", a: "", b: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotEqual(t, HashToken(tt.a), HashToken(tt.b))
		})
	}
}

func TestHashToken_EmptyIsStable(t *testing.T) {
	// Even empty input yields a stable, full-length hex digest.
	require.Equal(t, HashToken(""), HashToken(""))
	require.Len(t, HashToken(""), 64)
}
