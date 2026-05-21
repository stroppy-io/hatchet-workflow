// Package yandexcloud is the live Yandex Cloud adapter for quota admission (D19)
// and network inventory (D20). It reads the tenant's stored token/cloud/folder
// from settings, then queries the cloud directly (the cloud is the source of
// truth, H33): quota limits via the quota manager, used private IPs by listing
// compute instances. In-flight reservations (VMs not yet created) come from the
// persisted mirror so concurrent runs do not collide before terraform applies.
package yandexcloud

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/gopherex/xlog"
	compute "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	quotamanager "github.com/yandex-cloud/go-genproto/yandex/cloud/quotamanager/v1"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/yandex"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/netinventory"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

type settingsRepo = repository.ProtoRepository[
	models.SettingsItemAlias,
	models.SettingsItemColumnAlias,
	*models.SettingsItemScanner,
	*models.SettingsItem,
]

// Client implements inventory.CloudQuota and deploy.NetworkInventory over the
// live Yandex Cloud API.
type Client struct {
	*tracing.Entity
	items  *settingsRepo
	mirror *netinventory.Inventory
}

// New builds the adapter. mirror holds in-flight IP reservations.
func New(logger *xlog.Logger, executor exec.DB, mirror *netinventory.Inventory) *Client {
	return &Client{
		Entity: tracing.NewEntity(logger.AppendName("YandexCloud")),
		items: repository.NewProtoRepository(
			repository.NewScannerRepository(models.SettingsItems.Table, executor),
			models.SettingsItemConverter,
		),
		mirror: mirror,
	}
}

type auth struct{ token, cloudID, folderID string }

func (c *Client) auth(ctx context.Context, tenantID string) (auth, error) {
	items, err := c.items.Query(ctx, models.SettingsItems.SelectAll().Where(
		models.SettingsItems.TenantId.Eq(tenantID),
		models.SettingsItems.DeletedAt.IsNull(),
	))
	if err != nil {
		return auth{}, err
	}
	var a auth
	for _, it := range items {
		switch it.GetKey() {
		case models.SettingsItem_KEY_YANDEX_CLOUD_TOKEN:
			a.token = it.GetValue().GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_CLOUD_ID:
			a.cloudID = it.GetValue().GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_FOLDER_ID:
			a.folderID = it.GetValue().GetStringValue()
		}
	}
	if a.token == "" || a.folderID == "" {
		return auth{}, fmt.Errorf("yandexcloud: tenant %s missing token/folder settings", tenantID)
	}
	return a, nil
}

// FetchQuotas returns the tenant's live compute quota limits + usage (D19).
func (c *Client) FetchQuotas(ctx context.Context, tenantID string, _ bool) (*deployment.QuotaInventory, error) {
	a, err := c.auth(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sdk, err := yandex.Build(ctx, a.token)
	if err != nil {
		return nil, err
	}
	defer sdk.Shutdown(ctx) //nolint:errcheck

	resp, err := sdk.QuotaManager().QuotaLimit().List(ctx, &quotamanager.ListQuotaLimitsRequest{
		Resource: &quotamanager.Resource{Id: a.cloudID, Type: "resource-manager.cloud"},
		Service:  "compute",
	})
	if err != nil {
		return nil, fmt.Errorf("yandexcloud: list quotas: %w", err)
	}
	inv := &deployment.QuotaInventory{Provider: deployment.Provider_PROVIDER_YANDEX, FetchedAt: timestamppb.Now()}
	for _, ql := range resp.GetQuotaLimits() {
		limit := uint64(ql.GetLimit().GetValue())
		used := uint64(ql.GetUsage().GetValue())
		var avail uint64
		if limit > used {
			avail = limit - used
		}
		inv.Quotas = append(inv.Quotas, &deployment.Quota{
			Provider:        deployment.Provider_PROVIDER_YANDEX,
			Resource:        mapQuotaResource(ql.GetQuotaId()),
			Limit:           limit,
			Used:            used,
			Available:       avail,
			ProviderQuotaId: ql.GetQuotaId(),
		})
	}
	return inv, nil
}

// Reconcile refreshes quotas from the cloud. There is no quota mirror table
// (cloud = source of truth), so this fetches live and reports the counts.
func (c *Client) Reconcile(ctx context.Context, tenantID string) (*ui.ReconcileResponse, error) {
	inv, err := c.FetchQuotas(ctx, tenantID, true)
	if err != nil {
		return nil, err
	}
	return &ui.ReconcileResponse{QuotasRefreshed: uint32(len(inv.GetQuotas()))}, nil
}

// UsedIPs lists private IPs already attached to compute instances in the tenant's
// folder that fall within subnetCIDR, plus in-flight mirror reservations.
func (c *Client) UsedIPs(ctx context.Context, tenantID, subnetCIDR string) ([]string, error) {
	_, subnet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return nil, err
	}
	a, err := c.auth(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sdk, err := yandex.Build(ctx, a.token)
	if err != nil {
		return nil, err
	}
	defer sdk.Shutdown(ctx) //nolint:errcheck

	var ips []string
	pageToken := ""
	for {
		resp, err := sdk.Compute().Instance().List(ctx, &compute.ListInstancesRequest{
			FolderId: a.folderID, PageSize: 1000, PageToken: pageToken,
		})
		if err != nil {
			return nil, fmt.Errorf("yandexcloud: list instances: %w", err)
		}
		for _, inst := range resp.GetInstances() {
			for _, ni := range inst.GetNetworkInterfaces() {
				if ip := net.ParseIP(ni.GetPrimaryV4Address().GetAddress()); ip != nil && subnet.Contains(ip) {
					ips = append(ips, ip.String())
				}
			}
		}
		if pageToken = resp.GetNextPageToken(); pageToken == "" {
			break
		}
	}
	// In-flight reservations (VMs not yet created in the VPC).
	if mirrorIPs, err := c.mirror.UsedIPs(ctx, tenantID, subnetCIDR); err == nil {
		ips = append(ips, mirrorIPs...)
	}
	return ips, nil
}

// Reserve records in-flight reservations in the mirror (the live VPC only shows
// an IP once terraform creates the VM).
func (c *Client) Reserve(ctx context.Context, tenantID string, ips []string) error {
	return c.mirror.Reserve(ctx, tenantID, ips)
}

// mapQuotaResource maps a Yandex compute quota id to our typed resource class.
func mapQuotaResource(quotaID string) deployment.QuotaResource {
	q := strings.ToLower(quotaID)
	switch {
	case strings.Contains(q, "core"):
		return deployment.QuotaResource_QUOTA_RESOURCE_CORES
	case strings.Contains(q, "memory"):
		return deployment.QuotaResource_QUOTA_RESOURCE_MEMORY_GB
	case strings.Contains(q, "ssd"):
		return deployment.QuotaResource_QUOTA_RESOURCE_SSD_GB
	case strings.Contains(q, "hdd"):
		return deployment.QuotaResource_QUOTA_RESOURCE_HDD_GB
	case strings.Contains(q, "instance"):
		return deployment.QuotaResource_QUOTA_RESOURCE_INSTANCES
	case strings.Contains(q, "externalip") || strings.Contains(q, "publicip"):
		return deployment.QuotaResource_QUOTA_RESOURCE_EXTERNAL_IPS
	case strings.Contains(q, "network"):
		return deployment.QuotaResource_QUOTA_RESOURCE_NETWORKS
	case strings.Contains(q, "subnet"):
		return deployment.QuotaResource_QUOTA_RESOURCE_SUBNETS
	default:
		return deployment.QuotaResource_QUOTA_RESOURCE_UNSPECIFIED
	}
}
