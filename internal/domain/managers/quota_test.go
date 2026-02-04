package managers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	vlkeyExt "github.com/stroppy-io/hatchet-workflow/internal/infrastructure/valkey"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/valkey-io/valkey-go"
)

func testClient(t *testing.T) (valkey.Client, error) {
	client, err := vlkeyExt.NewValkey(&vlkeyExt.Config{
		Addresses: []string{"127.0.0.1:6379"},
		Username:  "",
		Password:  "developer",
	})
	if err != nil {
		return nil, err
	}
	// Flush the database before each test
	err = client.Do(context.Background(), client.B().Flushall().Build()).Error()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func TestQuotaManager_ReserveQuotas(t *testing.T) {
	ctx := context.Background()
	client, err := testClient(t)
	require.NoError(t, err)
	// Setup initial quotas
	config := DefaultQuotasConfig()
	// Reduce max for testing
	config.AvailableQuotas[0].Maximum = 2 // VM
	config.AvailableQuotas[1].Maximum = 2 // SUBNET

	manager, err := NewQuotaManager(client, config)
	require.NoError(t, err)

	// Test 1: Reserve within limits
	quotasToReserve := []*crossplane.Quota{
		{Cloud: crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX, Kind: crossplane.Quota_KIND_VM, Maximum: 1},
	}
	err = manager.ReserveQuotas(ctx, quotasToReserve)
	require.NoError(t, err)

	// Test 2: Reserve again within limits (1+1 = 2 <= 2)
	err = manager.ReserveQuotas(ctx, quotasToReserve)
	require.NoError(t, err)

	// Test 3: Reserve exceeding limits (2+1 = 3 > 2)
	err = manager.ReserveQuotas(ctx, quotasToReserve)
	require.Error(t, err)
	var quotaErr *errQuotaNotAvailable
	require.ErrorAs(t, err, &quotaErr)
	require.Equal(t, crossplane.Quota_KIND_VM, quotaErr.Quota.Kind)

	// Test 4: Free quotas
	err = manager.FreeQuotas(ctx, quotasToReserve)
	require.NoError(t, err)

	// Test 5: Reserve again after freeing (should succeed)
	err = manager.ReserveQuotas(ctx, quotasToReserve)
	require.NoError(t, err)
}

func TestQuotaManager_FreeQuotas(t *testing.T) {
	ctx := context.Background()
	client, err := testClient(t)
	require.NoError(t, err)

	config := DefaultQuotasConfig()
	manager, err := NewQuotaManager(client, config)
	require.NoError(t, err)

	quotas := []*crossplane.Quota{
		{Cloud: crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX, Kind: crossplane.Quota_KIND_VM, Maximum: 1},
	}

	// Reserve first
	err = manager.ReserveQuotas(ctx, quotas)
	require.NoError(t, err)

	// Free
	err = manager.FreeQuotas(ctx, quotas)
	require.NoError(t, err)

	// Verify we can reserve again (logic check mainly covered in previous test, but good for isolation)
	err = manager.ReserveQuotas(ctx, quotas)
	require.NoError(t, err)
}

func TestQuotaManager_Atomicity(t *testing.T) {
	// This test simulates a scenario where multiple quotas are requested,
	// and one fails. The operation should be atomic (all or nothing).
	// Note: With the current Lua script implementation, it checks all first, then decrements.
	// So if one fails check, none are decremented.

	ctx := context.Background()
	client, err := testClient(t)
	require.NoError(t, err)

	config := DefaultQuotasConfig()
	config.AvailableQuotas[0].Maximum = 1  // VM
	config.AvailableQuotas[1].Maximum = 10 // SUBNET

	manager, err := NewQuotaManager(client, config)
	require.NoError(t, err)

	// Request VM (1) and SUBNET (1). Available: VM=1, SUBNET=10. Should succeed.
	quotas := []*crossplane.Quota{
		{Cloud: crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX, Kind: crossplane.Quota_KIND_VM, Maximum: 1},
		{Cloud: crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX, Kind: crossplane.Quota_KIND_SUBNET, Maximum: 1},
	}
	err = manager.ReserveQuotas(ctx, quotas)
	require.NoError(t, err)

	// Request VM (1) and SUBNET (1). Available: VM=0, SUBNET=9. Should fail VM check.
	// SUBNET should NOT be decremented.
	err = manager.ReserveQuotas(ctx, quotas)
	require.Error(t, err)

	// Verify SUBNET count is still 9 (we consumed 1 in first step).
	// We can verify this by trying to consume remaining 9 SUBNETs.
	subnetOnly := []*crossplane.Quota{
		{Cloud: crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX, Kind: crossplane.Quota_KIND_SUBNET, Maximum: 9},
	}
	err = manager.ReserveQuotas(ctx, subnetOnly)
	require.NoError(t, err, "Should be able to reserve remaining 9 subnets if atomicity worked")
}
