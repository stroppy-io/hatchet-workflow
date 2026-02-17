package terraform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstallTerraform(t *testing.T) {
	actor, err := NewActor()
	require.NoError(t, err)
	require.NotEmpty(t, actor.execPath)
}
