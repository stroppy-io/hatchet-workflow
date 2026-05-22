// Package netinventory is the network-allocation mirror (D20): it records per-VM
// private-IP leases as /32 models.NetworkAllocation rows and reports the IPs
// currently in use so the allocator never collides across runs. Leases are
// TTL-bounded (no explicit free — expired leases are ignored), which bounds leak.
//
// This is the persisted (in-flight reservation) mirror; the live VPC view (the cloud
// as source of truth, H33) is services/yandexcloud, whose UsedIPs returns live
// instances ∪ this mirror. services/deploy reads used IPs through that union, so the
// subnet is never assumed empty.
package netinventory

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

type allocRepo = repository.ProtoRepository[
	models.NetworkAllocationAlias,
	models.NetworkAllocationColumnAlias,
	*models.NetworkAllocationScanner,
	*models.NetworkAllocation,
]

// Inventory reads and writes the IP-lease mirror.
type Inventory struct {
	*tracing.Entity
	repo *allocRepo
	ttl  time.Duration
}

// New builds an Inventory. ttl bounds how long a reserved IP stays "used".
func New(logger *xlog.Logger, executor exec.DB, ttl time.Duration) *Inventory {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Inventory{
		Entity: tracing.NewEntity(logger.AppendName("NetInventory")),
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(models.NetworkAllocations.Table, executor),
			models.NetworkAllocationConverter,
		),
		ttl: ttl,
	}
}

// UsedIPs returns the tenant's active leased IPs that fall within subnetCIDR.
func (i *Inventory) UsedIPs(ctx context.Context, tenantID, subnetCIDR string) ([]string, error) {
	_, subnet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return nil, err
	}
	rows, err := i.repo.Query(ctx, models.NetworkAllocations.SelectAll().Where(
		models.NetworkAllocations.TenantId.Eq(tenantID),
		models.NetworkAllocations.DeletedAt.IsNull(),
	))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var used []string
	for _, r := range rows {
		if exp := r.GetLeaseExpiresAt(); exp != nil && exp.AsTime().Before(now) {
			continue // lease expired -> free
		}
		ip := ipOfCIDR(r.GetCidr().GetValue())
		if ip != nil && subnet.Contains(ip) {
			used = append(used, ip.String())
		}
	}
	return used, nil
}

// Reserve records a TTL-bounded /32 lease per IP for the tenant.
func (i *Inventory) Reserve(ctx context.Context, tenantID string, ips []string) error {
	expires := timestamppb.New(time.Now().Add(i.ttl))
	for _, ip := range ips {
		na := &models.NetworkAllocation{
			Id:             ids.New(),
			TenantId:       &models.TenantId{Value: tenantID},
			Cidr:           &system.Cidr{Value: ip + "/32"},
			LeaseExpiresAt: expires,
			Timestamps:     &models.Timestamps{CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
			Tags:           &common.Tags{}, // default empty tags -> "{}" (NOT NULL serialized column)
		}
		scanner := na.IntoPlain()
		if _, err := i.repo.Scanner().Execute(ctx,
			models.NetworkAllocations.Insert().From(scanner.AllSetters()...)); err != nil {
			return err
		}
	}
	return nil
}

// ipOfCIDR extracts the host IP from a "<ip>/32" cidr (or a bare ip).
func ipOfCIDR(cidr string) net.IP {
	if cidr == "" {
		return nil
	}
	if idx := strings.IndexByte(cidr, '/'); idx >= 0 {
		cidr = cidr[:idx]
	}
	return net.ParseIP(cidr)
}
