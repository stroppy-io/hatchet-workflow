package identity

import (
	"golang.org/x/crypto/bcrypt"

	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// BcryptHasher implements iamsvc.PasswordHasher using golang.org/x/crypto/bcrypt.
type BcryptHasher struct {
	cost int
}

var _ iamsvc.PasswordHasher = (*BcryptHasher)(nil)

// NewBcryptHasher builds a hasher at the given cost. A cost <= 0 (or below
// bcrypt's minimum) falls back to bcrypt.DefaultCost.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

// Hash returns the bcrypt hash of the plaintext password.
func (h *BcryptHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify reports whether plain matches the stored bcrypt hash.
func (h *BcryptHasher) Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
