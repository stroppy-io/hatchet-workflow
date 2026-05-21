package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestSigner_SignAccount_RoundTrip(t *testing.T) {
	s := NewSigner([]byte("super-secret-key"))

	tok, err := s.SignAccount("account-123", true, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	claims, err := s.Parse(tok)
	require.NoError(t, err)
	require.Equal(t, KindAccount, claims.Kind)
	require.Equal(t, "account-123", claims.Subject)
	require.True(t, claims.IsAdmin)
}

func TestSigner_SignAgent_RoundTrip(t *testing.T) {
	s := NewSigner([]byte("super-secret-key"))

	tok, err := s.SignAgent("agent-1", "tenant-9", "machine-7", time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	claims, err := s.Parse(tok)
	require.NoError(t, err)
	require.Equal(t, KindAgent, claims.Kind)
	require.Equal(t, "agent-1", claims.AgentID)
	require.Equal(t, "tenant-9", claims.TenantID)
	require.Equal(t, "machine-7", claims.MachineID)
}

func TestSigner_Parse_RejectsWrongSecret(t *testing.T) {
	issuer := NewSigner([]byte("secret-A"))
	verifier := NewSigner([]byte("secret-B"))

	tok, err := issuer.SignAccount("account-123", false, time.Hour)
	require.NoError(t, err)

	_, err = verifier.Parse(tok)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestSigner_Parse_RejectsExpired(t *testing.T) {
	s := NewSigner([]byte("super-secret-key"))

	// Negative TTL => token already expired at issue time.
	tok, err := s.SignAccount("account-123", false, -time.Minute)
	require.NoError(t, err)

	_, err = s.Parse(tok)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestSigner_Parse_RejectsGarbage(t *testing.T) {
	s := NewSigner([]byte("super-secret-key"))

	tests := []struct {
		name  string
		token string
	}{
		{name: "empty", token: ""},
		{name: "plain garbage", token: "this-is-not-a-jwt"},
		{name: "malformed segments", token: "aaa.bbb.ccc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.Parse(tt.token)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}

func TestSigner_Parse_RejectsNoneAlg(t *testing.T) {
	s := NewSigner([]byte("super-secret-key"))

	// Forge a token signed with the "none" method; the parser must reject any
	// non-HMAC signing method.
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "attacker",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Kind: KindAccount,
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = s.Parse(tok)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidToken))
}
