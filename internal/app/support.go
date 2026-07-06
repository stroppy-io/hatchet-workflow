package app

import (
	"crypto/sha256"

	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

// commonEntity aliases the storage envelope the favorite target getters return,
// so the closures in run.go stay terse.
type commonEntity = common.Entity

// deriveSecretKey derives a fixed 32-byte AES key from the JWT signing secret so
// provider OIDC secrets can be sealed without configuring a second key. SHA-256
// yields exactly the 32 bytes AES-256 requires.
func deriveSecretKey(signingSecret string) []byte {
	sum := sha256.Sum256([]byte(signingSecret))
	return sum[:]
}
