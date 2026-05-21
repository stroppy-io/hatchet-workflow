package victoria

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSamplePairFloat(t *testing.T) {
	t.Run("valid string value", func(t *testing.T) {
		sp := SamplePair{1.6e9, "42.5"}
		v, ok := sp.Float()
		require.True(t, ok)
		require.Equal(t, 42.5, v)
	})

	t.Run("integer string value", func(t *testing.T) {
		sp := SamplePair{1.6e9, "100"}
		v, ok := sp.Float()
		require.True(t, ok)
		require.Equal(t, 100.0, v)
	})

	t.Run("scientific notation string", func(t *testing.T) {
		sp := SamplePair{1.6e9, "1.5e3"}
		v, ok := sp.Float()
		require.True(t, ok)
		require.Equal(t, 1500.0, v)
	})

	t.Run("non-numeric string -> false", func(t *testing.T) {
		sp := SamplePair{1.6e9, "not-a-number"}
		v, ok := sp.Float()
		require.False(t, ok)
		require.Equal(t, 0.0, v)
	})

	t.Run("value not a string (numeric) -> false", func(t *testing.T) {
		sp := SamplePair{1.6e9, 42.5}
		v, ok := sp.Float()
		require.False(t, ok)
		require.Equal(t, 0.0, v)
	})

	t.Run("nil value -> false", func(t *testing.T) {
		sp := SamplePair{1.6e9, nil}
		v, ok := sp.Float()
		require.False(t, ok)
		require.Equal(t, 0.0, v)
	})

	t.Run("empty string -> false", func(t *testing.T) {
		sp := SamplePair{1.6e9, ""}
		v, ok := sp.Float()
		require.False(t, ok)
		require.Equal(t, 0.0, v)
	})
}
