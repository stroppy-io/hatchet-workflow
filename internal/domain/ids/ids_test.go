package ids_test

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
)

func TestNewNonEmpty(t *testing.T) {
	id := ids.New()
	require.NotEmpty(t, id)
	// ULID string form is 26 chars (Crockford base32).
	require.Len(t, id, 26)
}

func TestNewUnique(t *testing.T) {
	const n = 10000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := ids.New()
		require.NotEmpty(t, id)
		_, dup := seen[id]
		require.Falsef(t, dup, "duplicate id generated: %s", id)
		seen[id] = struct{}{}
	}
	require.Len(t, seen, n)
}

// TestNewSortableByTime asserts ULIDs generated in time order sort
// lexicographically in roughly the same order (the 48-bit ms timestamp prefix
// dominates the ordering across time boundaries).
func TestNewSortableByTime(t *testing.T) {
	first := ids.New()
	time.Sleep(2 * time.Millisecond)
	second := ids.New()
	time.Sleep(2 * time.Millisecond)
	third := ids.New()

	sorted := []string{first, second, third}
	require.True(t, sort.StringsAreSorted(sorted),
		"ULIDs minted in time order must be lexicographically sorted: %v", sorted)
}

func TestNewUlid(t *testing.T) {
	u := ids.NewUlid()
	require.NotNil(t, u)
	require.NotEmpty(t, u.GetValue())
	require.Len(t, u.GetValue(), 26)

	other := ids.NewUlid()
	require.NotEqual(t, u.GetValue(), other.GetValue())
}

func TestNewEntity(t *testing.T) {
	before := time.Now().Add(-time.Second)

	e := ids.NewEntity()
	require.NotNil(t, e)

	// Id set and well-formed.
	require.NotNil(t, e.GetId())
	require.NotEmpty(t, e.GetId().GetValue())
	require.Len(t, e.GetId().GetValue(), 26)

	// Timestamps set, created == updated, and both ~ now.
	ts := e.GetTimestamps()
	require.NotNil(t, ts)
	require.NotNil(t, ts.GetCreatedAt())
	require.NotNil(t, ts.GetUpdatedAt())
	require.Nil(t, ts.GetDeletedAt())

	created := ts.GetCreatedAt().AsTime()
	updated := ts.GetUpdatedAt().AsTime()
	require.Equal(t, created, updated, "new entity created/updated should be equal")

	after := time.Now().Add(time.Second)
	require.True(t, created.After(before) && created.Before(after),
		"created timestamp should be ~now: %v", created)
}

func TestNewEntityUnique(t *testing.T) {
	a := ids.NewEntity()
	b := ids.NewEntity()
	require.NotEqual(t, a.GetId().GetValue(), b.GetId().GetValue())
}
