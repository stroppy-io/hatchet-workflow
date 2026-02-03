package managers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetworkManager_ReserveNetwork(t *testing.T) {
	ctx := context.Background()
	client, err := testClient(t)
	require.NoError(t, err)

	// Clean up keys before test
	_ = client.Do(ctx, client.B().Del().Key(networksKey).Build()).Error()
	_ = client.Do(ctx, client.B().Del().Key(networkLockKey).Build()).Error()

	config := DefaultNetworkConfig()
	// Use a smaller range for testing if needed, but default is fine
	// BaseCidr: "10.2.0.0/16", BasePrefix: 24

	manager, err := NewNetworkManager(client, config)
	require.NoError(t, err)

	// Test 1: Reserve first network
	net1, err := manager.ReserveNetwork(ctx, 3)
	require.NoError(t, err)
	require.NotNil(t, net1)
	require.NotNil(t, net1.Cidr)
	require.Equal(t, "10.2.0.0/24", net1.Cidr.Value)
	require.Len(t, net1.Ips, 3)
	// Check IPs are within range and padded
	// Padding is 3, so first IP should be .4
	require.Equal(t, "10.2.0.4", net1.Ips[0].Value)
	require.Equal(t, "10.2.0.5", net1.Ips[1].Value)
	require.Equal(t, "10.2.0.6", net1.Ips[2].Value)

	// Test 2: Reserve second network
	net2, err := manager.ReserveNetwork(ctx, 2)
	require.NoError(t, err)
	require.NotNil(t, net2)
	require.Equal(t, "10.2.1.0/24", net2.Cidr.Value)
	require.Len(t, net2.Ips, 2)
	require.Equal(t, "10.2.1.4", net2.Ips[0].Value)
	require.Equal(t, "10.2.1.5", net2.Ips[1].Value)

	// Test 3: Free first network
	err = manager.FreeNetwork(ctx, net1)
	require.NoError(t, err)

	// Test 4: Reserve again, should reuse the freed space (10.2.0.0/24)
	// Note: NextSubnetWithIPs logic iterates to find first free gap.
	// Since we freed 10.2.0.0/24, it should be available again.
	net3, err := manager.ReserveNetwork(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "10.2.0.0/24", net3.Cidr.Value)
	require.Len(t, net3.Ips, 1)
	require.Equal(t, "10.2.0.4", net3.Ips[0].Value)
}

func TestNetworkManager_FreeNetwork(t *testing.T) {
	ctx := context.Background()
	client, err := testClient(t)
	require.NoError(t, err)

	// Clean up
	_ = client.Do(ctx, client.B().Del().Key(networksKey).Build()).Error()

	config := DefaultNetworkConfig()
	manager, err := NewNetworkManager(client, config)
	require.NoError(t, err)

	// Reserve
	net1, err := manager.ReserveNetwork(ctx, 3)
	require.NoError(t, err)

	// Verify it's in the set
	members, err := client.Do(ctx, client.B().Smembers().Key(networksKey).Build()).AsStrSlice()
	require.NoError(t, err)
	require.Contains(t, members, net1.Cidr.Value)

	// Free
	err = manager.FreeNetwork(ctx, net1)
	require.NoError(t, err)

	// Verify it's gone
	members, err = client.Do(ctx, client.B().Smembers().Key(networksKey).Build()).AsStrSlice()
	require.NoError(t, err)
	require.NotContains(t, members, net1.Cidr.Value)
}

func TestNetworkManager_Concurrency(t *testing.T) {
	// This test attempts to run multiple reservations concurrently to ensure locking works
	// and no duplicate subnets are handed out.
	ctx := context.Background()
	client, err := testClient(t)
	require.NoError(t, err)

	_ = client.Do(ctx, client.B().Del().Key(networksKey).Build()).Error()
	_ = client.Do(ctx, client.B().Del().Key(networkLockKey).Build()).Error()

	config := DefaultNetworkConfig()
	manager, err := NewNetworkManager(client, config)
	require.NoError(t, err)

	concurrency := 10
	results := make(chan string, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			net, err := manager.ReserveNetwork(ctx, 1)
			if err != nil {
				errors <- err
				return
			}
			results <- net.Cidr.Value
		}()
	}

	seen := make(map[string]bool)
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-errors:
			t.Fatalf("concurrent reservation failed: %v", err)
		case cidr := <-results:
			if seen[cidr] {
				t.Fatalf("duplicate CIDR allocated: %s", cidr)
			}
			seen[cidr] = true
		}
	}
	require.Len(t, seen, concurrency)
}
