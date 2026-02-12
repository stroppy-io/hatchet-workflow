package managers

import (
	"context"
	"fmt"
	"time"

	"github.com/stroppy-io/hatchet-workflow/internal/core/ips"
	"github.com/stroppy-io/hatchet-workflow/internal/infrastructure/valkey"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	valkeygo "github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeylock"
)

const (
	networkLockKey = "network_manager_lock"
	networksKey    = "reserved_networks"
)

type NetworkManager struct {
	valkeyClient valkeygo.Client
	locker       valkeylock.Locker
}

func NewNetworkManager(valkeyClient valkeygo.Client) (*NetworkManager, error) {
	locker, err := valkey.NewValkeyLocker(valkeyClient, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey locker: %w", err)
	}

	return &NetworkManager{
		valkeyClient: valkeyClient,
		locker:       locker,
	}, nil
}

func (n NetworkManager) ReserveNetwork(
	ctx context.Context,
	networkIdentifier *deployment.Identifier,
	baseCidr string,
	basePrefix int,
	ipCount int,
) (*deployment.Network, error) {
	if ipCount <= 0 {
		return nil, fmt.Errorf("ip count must be greater than 0")
	}
	if baseCidr == "" {
		return nil, fmt.Errorf("base cidr must be specified")
	}
	if basePrefix <= 0 || basePrefix > 32 {
		return nil, fmt.Errorf("base prefix must be between 1 and 32")
	}
	if networkIdentifier == nil {
		return nil, fmt.Errorf("network identifier is nil")
	}

	// Acquire lock to prevent race conditions
	networkKey := fmt.Sprintf(
		"%s:%s:%s",
		networkLockKey,
		networkIdentifier.GetTarget().String(),
		networkIdentifier.GetName(),
	)
	_, unlock, err := n.locker.WithContext(ctx, networkKey)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire network lock: %w", err)
	}
	defer unlock()

	// Get existing networks
	existingNetworks, err := n.valkeyClient.Do(ctx, n.valkeyClient.B().
		Smembers().Key(networkKey).
		Build()).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to get existing networks: %w", err)
	}

	subnet, ipsList, err := ips.NextSubnetWithIPs(baseCidr, basePrefix, existingNetworks, ipCount)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate subnet: %w", err)
	}

	// Create the subnet template object
	subnetTemplate := &deployment.Network{
		Identifier: networkIdentifier,
		Cidr: &deployment.Cidr{
			Value: subnet.String(),
		},
		Ips: make([]*deployment.Ip, len(ipsList)),
	}

	for i, ip := range ipsList {
		subnetTemplate.Ips[i] = &deployment.Ip{
			Value: ip.String(),
		}
	}

	// Save the new subnet to the set of reserved networks
	added, err := n.valkeyClient.Do(ctx, n.valkeyClient.B().
		Sadd().Key(networkKey).
		Member(subnet.String()).
		Build()).AsInt64()
	if err != nil {
		return nil, fmt.Errorf("failed to save reserved network: %w", err)
	}
	if added == 0 {
		return nil, fmt.Errorf("failed to reserve network: subnet %s already reserved", subnet.String())
	}

	return subnetTemplate, nil
}

func (n NetworkManager) FreeNetwork(
	ctx context.Context,
	network *deployment.Network,
) error {
	if err := network.Validate(); err != nil {
		return fmt.Errorf("invalid network: %w", err)
	}
	storageKey := fmt.Sprintf(
		"%s:%s:%s",
		networksKey,
		network.GetIdentifier().GetTarget().String(),
		network.GetIdentifier().GetName(),
	)
	// Remove from reserved networks
	err := n.valkeyClient.Do(ctx, n.valkeyClient.B().
		Srem().Key(storageKey).
		Member(network.GetCidr().GetValue()).
		Build()).Error()
	if err != nil {
		return fmt.Errorf("failed to free network: %w", err)
	}

	return nil
}
