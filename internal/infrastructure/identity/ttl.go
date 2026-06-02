package identity

import (
	"time"

	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// TokenTTL implements iamsvc.TokenTTL, sourcing the one-time-token lifetimes
// from Config.
type TokenTTL struct {
	emailVerification time.Duration
	passwordReset     time.Duration
}

var _ iamsvc.TokenTTL = (*TokenTTL)(nil)

// NewTokenTTL builds the TTL provider from config (defaults applied).
func NewTokenTTL(cfg Config) *TokenTTL {
	return &TokenTTL{
		emailVerification: cfg.emailVerificationTTL(),
		passwordReset:     cfg.passwordResetTTL(),
	}
}

func (t *TokenTTL) EmailVerificationTTL() time.Duration { return t.emailVerification }

func (t *TokenTTL) PasswordResetTTL() time.Duration { return t.passwordReset }
