package passwd

import (
	"crypto/sha256"
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"
)

/*
	passwd is the bcrypt PasswordHasher implementation. bcrypt silently truncates
	input past 72 bytes, so long passwords (the proto allows up to 1024) would
	collide on their first 72 bytes. To avoid that, the plaintext is first folded
	to a fixed 44-char base64 SHA-256 digest, then bcrypt-hashed — full-length
	passwords stay distinct and within bcrypt's input limit.
*/

type BcryptHasher struct {
	cost int
}

// NewBcryptHasher builds a hasher at the given cost; a non-positive cost falls
// back to bcrypt.DefaultCost.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

func (h *BcryptHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword(prehash(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *BcryptHasher) Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), prehash(plain)) == nil
}

func prehash(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return []byte(base64.StdEncoding.EncodeToString(sum[:]))
}
