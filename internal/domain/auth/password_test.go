package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	const pw = "correct horse battery staple"

	hash, err := HashPassword(pw)
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	// The stored hash must never be the plaintext.
	require.NotEqual(t, pw, hash)

	// Round-trip: the right password verifies.
	require.True(t, VerifyPassword(hash, pw))
}

func TestVerifyPassword(t *testing.T) {
	const pw = "s3cret-passphrase"
	hash, err := HashPassword(pw)
	require.NoError(t, err)

	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{name: "correct password", password: pw, want: true},
		{name: "wrong password", password: "s3cret-passphras", want: false},
		{name: "empty password", password: "", want: false},
		{name: "case-sensitive mismatch", password: "S3CRET-PASSPHRASE", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, VerifyPassword(hash, tt.password))
		})
	}
}

func TestVerifyPassword_GarbageHash(t *testing.T) {
	// A non-bcrypt hash must not verify and must not panic.
	require.False(t, VerifyPassword("not-a-bcrypt-hash", "anything"))
	require.False(t, VerifyPassword("", "anything"))
}

func TestHashPassword_SaltDiffersPerCall(t *testing.T) {
	const pw = "same-input-each-time"

	h1, err := HashPassword(pw)
	require.NoError(t, err)
	h2, err := HashPassword(pw)
	require.NoError(t, err)

	// bcrypt embeds a random salt, so two hashes of the same input differ,
	// yet both must still verify against the original password.
	require.NotEqual(t, h1, h2)
	require.True(t, VerifyPassword(h1, pw))
	require.True(t, VerifyPassword(h2, pw))
}

func TestVerifyPasswordConstantTime(t *testing.T) {
	const pw = "machine-secret"
	hash, err := HashPassword(pw)
	require.NoError(t, err)

	t.Run("correct password with present hash", func(t *testing.T) {
		require.True(t, VerifyPasswordConstantTime(&hash, pw))
	})

	t.Run("wrong password with present hash", func(t *testing.T) {
		require.False(t, VerifyPasswordConstantTime(&hash, "wrong"))
	})

	t.Run("nil hash returns false", func(t *testing.T) {
		// Account-not-found path: must return false without panicking.
		require.False(t, VerifyPasswordConstantTime(nil, pw))
	})
}
