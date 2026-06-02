package identity

import "time"

// Config carries the crypto/identity tunables for the identity adapters. It is
// supplied by the integration layer (typically from app config). Zero-value
// fields fall back to the documented defaults at construction time.
type Config struct {
	// SigningSecret is the HMAC (HS256) secret used to sign and verify the
	// short-lived access tokens AND the server-side refresh tokens. It MUST be
	// non-empty in production; an empty secret is rejected by the constructors.
	SigningSecret string
	// Issuer is the JWT "iss" claim stamped on minted tokens and required on
	// verification. Empty -> "stroppy.io".
	Issuer string
	// AccessTTL is the access-token lifetime. Empty/zero -> 15m.
	AccessTTL time.Duration
	// RefreshTTL is the refresh-token (session) lifetime. Empty/zero -> 720h.
	RefreshTTL time.Duration
	// EmailVerificationTTL is the one-time email-verification token lifetime.
	// Empty/zero -> 24h.
	EmailVerificationTTL time.Duration
	// PasswordResetTTL is the one-time password-reset token lifetime.
	// Empty/zero -> 1h.
	PasswordResetTTL time.Duration
	// SecretEncryptionKey is the symmetric key used to seal stored secrets
	// (OIDC client secrets). It MUST be exactly 16, 24 or 32 bytes for AES.
	// Empty -> the secret stores are constructed with a clear typed error.
	SecretEncryptionKey []byte
	// BcryptCost is the bcrypt work factor. Out-of-range -> bcrypt.DefaultCost.
	BcryptCost int
	// SSORedirectBaseURL is the public base URL the IdP redirects back to after
	// authentication, e.g. "https://app.example.com". The per-provider callback
	// path /auth/sso/<slug>/callback is appended. Empty -> SSO Authorize fails
	// with a typed FailedPrecondition error.
	SSORedirectBaseURL string
}

const (
	defaultIssuer               = "stroppy.io"
	defaultAccessTTL            = 15 * time.Minute
	defaultRefreshTTL           = 720 * time.Hour
	defaultEmailVerificationTTL = 24 * time.Hour
	defaultPasswordResetTTL     = time.Hour
)

func (c Config) issuer() string {
	if c.Issuer == "" {
		return defaultIssuer
	}
	return c.Issuer
}

func (c Config) accessTTL() time.Duration {
	if c.AccessTTL <= 0 {
		return defaultAccessTTL
	}
	return c.AccessTTL
}

func (c Config) refreshTTL() time.Duration {
	if c.RefreshTTL <= 0 {
		return defaultRefreshTTL
	}
	return c.RefreshTTL
}

func (c Config) emailVerificationTTL() time.Duration {
	if c.EmailVerificationTTL <= 0 {
		return defaultEmailVerificationTTL
	}
	return c.EmailVerificationTTL
}

func (c Config) passwordResetTTL() time.Duration {
	if c.PasswordResetTTL <= 0 {
		return defaultPasswordResetTTL
	}
	return c.PasswordResetTTL
}
