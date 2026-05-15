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

// New returns a new monotonically increasing ULID string (26 characters).
func New() string {
	mu.Lock()
	id := ulid.MustNew(ulid.Now(), entropy)
	mu.Unlock()
	return id.String()
}
