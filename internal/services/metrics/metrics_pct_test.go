package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPct(t *testing.T) {
	t.Run("zero base -> 0", func(t *testing.T) {
		require.Equal(t, 0.0, pct(0, 100))
		require.Equal(t, 0.0, pct(0, 0))
	})

	t.Run("positive increase", func(t *testing.T) {
		require.Equal(t, 50.0, pct(100, 150))
	})

	t.Run("positive decrease (negative pct)", func(t *testing.T) {
		require.Equal(t, -50.0, pct(100, 50))
	})

	t.Run("no change -> 0", func(t *testing.T) {
		require.Equal(t, 0.0, pct(100, 100))
	})

	t.Run("doubling -> 100", func(t *testing.T) {
		require.Equal(t, 100.0, pct(50, 100))
	})

	t.Run("negative base", func(t *testing.T) {
		// (b - a) / a * 100 = (-50 - -100) / -100 * 100 = 50 / -100 * 100 = -50
		require.Equal(t, -50.0, pct(-100, -50))
	})

	t.Run("fractional", func(t *testing.T) {
		require.InDelta(t, 33.333333, pct(3, 4), 1e-6)
	})
}
