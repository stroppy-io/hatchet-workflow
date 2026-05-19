package ids

import (
	"crypto/rand"
	"sync"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

// Placeholder is a 26-char zero string clients send on Create-style RPCs
// when they cannot supply a real ID. Server code substitutes a fresh ULID
// (for entity own-ids) or a sensible default like the calling user.
const Placeholder = "00000000000000000000000000"

// IsPlaceholder reports whether the given ID equals the 26-char zero placeholder.
func IsPlaceholder(id string) bool { return id == Placeholder }

// New returns a new monotonically increasing ULID string (26 characters).
func New() string {
	mu.Lock()
	id := ulid.MustNew(ulid.Now(), entropy)
	mu.Unlock()
	return id.String()
}
