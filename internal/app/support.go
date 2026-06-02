package app

import (
	"crypto/sha256"
	"path/filepath"
	"strings"

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

// probeBinaryCacheDir derives the directory the stroppy prober caches downloaded
// stroppy binaries under, nested beside the gateway's own binary cache so probe
// and gateway artifacts share one configurable root (cfg.CacheDir). An empty
// root falls back to a per-OS temp dir owned by the prober.
func probeBinaryCacheDir(cacheDir string) string {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return ""
	}
	return filepath.Join(cacheDir, "stroppy-probe-binaries")
}
