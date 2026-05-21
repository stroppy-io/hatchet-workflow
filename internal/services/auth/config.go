// Package auth implements the AuthService (Login/RefreshTokens/Logout/Me) and the
// middleware.Authenticator (3-principal resolution). Refresh sessions live in
// Valkey with a TTL; the access token is a stateless JWT. Pure primitives (jwt,
// password, opaque tokens) live in internal/domain/auth.
package auth

import "time"

// Config provides AuthService tunables. Implementations may hot-reload values.
type Config interface {
	// AccessTokenTTL is the lifetime of the stateless access JWT.
	AccessTokenTTL() time.Duration
	// RefreshTokenTTL is the lifetime of a refresh session in Valkey.
	RefreshTokenTTL() time.Duration
	// JWTSecret is the HMAC secret for signing/verifying access tokens.
	JWTSecret() []byte
}
