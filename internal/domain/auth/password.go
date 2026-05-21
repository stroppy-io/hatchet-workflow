// Package auth holds the pure, I/O-free auth primitives: password hashing,
// JWT signing/parsing, and opaque-token generation/hashing. Orchestration
// (repositories, Valkey sessions, transactions) lives in internal/services/auth.
package auth

import "golang.org/x/crypto/bcrypt"

// dummyHash is a pre-computed bcrypt hash used to keep password verification
// constant-time even when the account does not exist (prevents a timing oracle
// on account existence).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-oracle-dummy"), bcrypt.DefaultCost)

// HashPassword returns the bcrypt hash of password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword reports whether password matches the bcrypt hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// VerifyPasswordConstantTime verifies password against hash, running a bcrypt
// comparison even when hash is nil (account not found) so the response time does
// not reveal account existence. Returns false when hash is nil.
func VerifyPasswordConstantTime(hash *string, password string) bool {
	if hash == nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return false
	}
	return VerifyPassword(*hash, password)
}
