package managers

import (
	"context"
	"fmt"
	"time"

	"github.com/stroppy-io/hatchet-workflow/internal/core/ips"
	"github.com/stroppy-io/hatchet-workflow/internal/infrastructure/valkey"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	valkeygo "github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeylock"
)

const (
	networkLockKey = "network_manager_lock"
	networksKey    = "reserved_networks"
)

type NetworkConfig struct {
	BaseCidr   string `mapstructure:"base_cidr"`
	BasePrefix int    `mapstructure:"base_prefix"`
}

func DefaultNetworkConfig() *NetworkConfig {
	return &NetworkConfig{
		BaseCidr:   "10.2.0.0/16",
		BasePrefix: 24,
	}
}

type NetworkManager struct {
	valkeyClient valkeygo.Client
	locker       valkeylock.Locker
	config       *NetworkConfig
}

func NewNetworkManager(valkeyClient valkeygo.Client, config *NetworkConfig) (*NetworkManager, error) {
	if config == nil {
		return nil, fmt.Errorf("network config is nil")
	}

	locker, err := valkey.NewValkeyLocker(valkeyClient, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey locker: %w", err)
	}

	return &NetworkManager{
		valkeyClient: valkeyClient,
		locker:       locker,
		config:       config,
	}, nil
}

func (n NetworkManager) ReserveNetwork(ctx context.Context, ipCount int) (*crossplane.CidrWithIps, error) {
	// Acquire lock to prevent race conditions
	// WithContext waits for the lock.
	// It returns a context that is canceled when the lock is lost or released.
	// We should use this context for operations that require the lock, but here we just need to hold it
	// while we read and write to Valkey.
	_, unlock, err := n.locker.WithContext(ctx, networkLockKey)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire network lock: %w", err)
	}
	defer unlock()

	// Get existing networks
	existingNetworks, err := n.valkeyClient.Do(ctx, n.valkeyClient.B().Smembers().Key(networksKey).Build()).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to get existing networks: %w", err)
	}

	if ipCount <= 0 {
		ipCount = 3 // Default to 3 IPs if not specified
	}

	subnet, ipsList, err := ips.NextSubnetWithIPs(n.config.BaseCidr, n.config.BasePrefix, existingNetworks, ipCount)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate subnet: %w", err)
	}

	// Create the network object
	network := &crossplane.CidrWithIps{
		Cidr: &crossplane.Cidr{
			Value: subnet.String(),
		},
		Ips: make([]*crossplane.Ip, len(ipsList)),
	}

	for i, ip := range ipsList {
		network.Ips[i] = &crossplane.Ip{
			Value: ip.String(),
		}
	}

	// Save the new subnet to the set of reserved networks
	err = n.valkeyClient.Do(ctx, n.valkeyClient.B().Sadd().Key(networksKey).Member(subnet.String()).Build()).Error()
	if err != nil {
		return nil, fmt.Errorf("failed to save reserved network: %w", err)
	}

	return network, nil
}

func (n NetworkManager) FreeNetwork(ctx context.Context, network *crossplane.CidrWithIps) error {
	if network == nil || network.Cidr == nil || network.Cidr.Value == "" {
		return nil // Nothing to free
	}

	// Remove from reserved networks
	err := n.valkeyClient.Do(ctx, n.valkeyClient.B().Srem().Key(networksKey).Member(network.Cidr.Value).Build()).Error()
	if err != nil {
		return fmt.Errorf("failed to free network: %w", err)
	}

	return nil
}
