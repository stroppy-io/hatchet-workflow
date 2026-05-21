package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// opaqueTokenBytes is the entropy of a generated opaque token (refresh / API token).
const opaqueTokenBytes = 32

// GenerateOpaqueToken returns a new high-entropy URL-safe opaque token. Used for
// refresh tokens and API tokens — the plaintext is shown once; only HashToken of
// it is persisted.
func GenerateOpaqueToken() (string, error) {
	b := make([]byte, opaqueTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns the hex sha256 of an opaque token, suitable for storage and
// constant-shape lookup (refresh-session key, ApiToken.token_hash).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
