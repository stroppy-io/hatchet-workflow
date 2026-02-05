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
	networkIdentifier *crossplane.Identifier,
	subnetIdentifier *crossplane.Identifier,
	baseCidr string,
	basePrefix int,
	ipCount int,
) (*crossplane.Subnet_Template, error) {
	if networkIdentifier == nil {
		return nil, fmt.Errorf("network identifier is nil")
	}

	// Acquire lock to prevent race conditions
	lockKey := fmt.Sprintf("%s:%s:%s", networkLockKey, networkIdentifier.GetSupportedCloud().String(), networkIdentifier.GetName())
	_, unlock, err := n.locker.WithContext(ctx, lockKey)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire network lock: %w", err)
	}
	defer unlock()

	// Get existing networks
	storageKey := fmt.Sprintf("%s:%s:%s", networksKey, networkIdentifier.GetSupportedCloud().String(), networkIdentifier.GetName())
	existingNetworks, err := n.valkeyClient.Do(ctx, n.valkeyClient.B().Smembers().Key(storageKey).Build()).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to get existing networks: %w", err)
	}

	if ipCount <= 0 {
		ipCount = 3 // Default to 3 IPs if not specified
	}

	subnet, ipsList, err := ips.NextSubnetWithIPs(baseCidr, basePrefix, existingNetworks, ipCount)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate subnet: %w", err)
	}

	// Create the subnet template object
	subnetTemplate := &crossplane.Subnet_Template{
		Identifier: subnetIdentifier,
		Cidr: &crossplane.Cidr{
			Value: subnet.String(),
		},
		Ips: make([]*crossplane.Ip, len(ipsList)),
	}

	for i, ip := range ipsList {
		subnetTemplate.Ips[i] = &crossplane.Ip{
			Value: ip.String(),
		}
	}

	// Save the new subnet to the set of reserved networks
	added, err := n.valkeyClient.Do(ctx, n.valkeyClient.B().
		Sadd().
		Key(storageKey).
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
	networkIdentifier *crossplane.Identifier,
	subnet *crossplane.Subnet_Template,
) error {
	if networkIdentifier == nil {
		return fmt.Errorf("network identifier is nil")
	}
	if subnet == nil || subnet.Cidr == nil || subnet.Cidr.Value == "" {
		return nil // Nothing to free
	}

	storageKey := fmt.Sprintf("%s:%s:%s", networksKey, networkIdentifier.GetSupportedCloud().String(), networkIdentifier.GetName())
	// Remove from reserved networks
	err := n.valkeyClient.Do(ctx, n.valkeyClient.B().Srem().Key(storageKey).Member(subnet.Cidr.Value).Build()).Error()
	if err != nil {
		return fmt.Errorf("failed to free network: %w", err)
	}

	return nil
}

func (n NetworkManager) FreeNetworkMany(
	ctx context.Context,
	networkIdentifier *crossplane.Identifier,
	subnets []*crossplane.Subnet_Template,
) error {
	if networkIdentifier == nil {
		return fmt.Errorf("network identifier is nil")
	}
	if len(subnets) == 0 {
		return nil
	}

	storageKey := fmt.Sprintf("%s:%s:%s", networksKey, networkIdentifier.GetSupportedCloud().String(), networkIdentifier.GetName())

	members := make([]string, 0, len(subnets))
	for _, subnet := range subnets {
		if subnet != nil && subnet.Cidr != nil && subnet.Cidr.Value != "" {
			members = append(members, subnet.Cidr.Value)
		}
	}

	if len(members) == 0 {
		return nil
	}

	// Remove from reserved networks in one go
	err := n.valkeyClient.Do(ctx, n.valkeyClient.B().Srem().Key(storageKey).Member(members...).Build()).Error()
	if err != nil {
		return fmt.Errorf("failed to free networks: %w", err)
	}

	return nil
}
