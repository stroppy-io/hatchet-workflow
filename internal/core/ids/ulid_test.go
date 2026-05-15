package ids_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
)

func TestNewULIDIs26Chars(t *testing.T) {
	id := ids.New()
	if len(id) != 26 {
		t.Fatalf("expected 26 chars, got %d: %q", len(id), id)
	}
}

func TestULIDsMonotonic(t *testing.T) {
	const n = 100
	prev := ids.New()
	for i := 0; i < n; i++ {
		next := ids.New()
		if next <= prev {
			t.Fatalf("ULIDs not monotonically increasing: %s >= %s", next, prev)
		}
		prev = next
	}
}
