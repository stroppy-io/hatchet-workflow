package schemas

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// TestAll: every schema builds, compiles, has a unique public id and passes
// the descriptor check; the registry accepts all of them.
func TestAll(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range All() {
		id := ids.Public(s.GetId())
		if seen[id] {
			t.Errorf("duplicate public id %s", id)
		}
		seen[id] = true
		if err := s.CheckDescriptor(); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
	if _, err := Registry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(seen) < 50 {
		t.Errorf("only %d schemas registered", len(seen))
	}
}
