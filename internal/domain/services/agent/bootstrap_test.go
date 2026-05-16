package agent_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
)

func TestBootstrapTokenStore_IssueAndVerify(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-pad!!!")
	store := agentsvc.NewBootstrapTokenStore(secret)

	tok, err := store.Issue("tenant-1", "dag-run-1", "machine-1", "database")
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	claims, err := store.Verify(tok)
	require.NoError(t, err)
	require.Equal(t, "tenant-1", claims.TenantID)
	require.Equal(t, "dag-run-1", claims.DagRunID)
	require.Equal(t, "machine-1", claims.MachineID)
	require.Equal(t, "database", claims.Role)
}

func TestBootstrapTokenStore_RejectsBadSecret(t *testing.T) {
	store1 := agentsvc.NewBootstrapTokenStore([]byte("secret-a-32-bytes-long-padding!!"))
	store2 := agentsvc.NewBootstrapTokenStore([]byte("secret-b-32-bytes-long-padding!!"))

	tok, err := store1.Issue("t", "d", "m", "r")
	require.NoError(t, err)

	_, err = store2.Verify(tok)
	require.Error(t, err)
}
