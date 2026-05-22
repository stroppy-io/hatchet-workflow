// Package provider resolves a tenant's Yandex Cloud settings (SettingsService) into
// the proto deployment.Yandex pieces (Network + Compute + a per-VM defaults template)
// — the Yandex.Input proto IS the terraform variables (deployment/yandex.proto).
// Settings enums (platform/zone/disk/accel) are translated to the terraform string
// forms here via the yandex module's canonical mappers.
package provider

import (
	"context"
	"fmt"

	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/deployments/terraform/yandex"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// Resolver reads tenant SettingsItems and builds DeploymentParams.
type Resolver struct {
	*tracing.Entity
	items *repository.ProtoRepository[
		models.SettingsItemAlias,
		models.SettingsItemColumnAlias,
		*models.SettingsItemScanner,
		*models.SettingsItem,
	]
}

// New builds a Resolver over the given DB executor.
func New(logger *xlog.Logger, executor exec.DB) *Resolver {
	return &Resolver{
		Entity: tracing.NewEntity(logger.AppendName("ProviderResolver")),
		items: repository.NewProtoRepository(
			repository.NewScannerRepository(models.SettingsItems.Table, executor),
			models.SettingsItemConverter,
		),
	}
}

// Resolve reads the tenant's Yandex Cloud settings and returns the deployment.Yandex
// pieces: the Network, the Compute base (platform/image), and a per-VM defaults
// template (zone/boot-disk-type/network-acceleration/public-ip) that services/deploy
// clones per machine (filling cores/mem/disk + internal_ip + user_data). The
// Yandex.Input proto these compose IS the terraform tfvars.
func (r *Resolver) Resolve(ctx context.Context, tenantID string) (*deployment.Yandex_Network, *deployment.Yandex_Compute, *deployment.Yandex_Vm, error) {
	items, err := r.items.Query(ctx, models.SettingsItems.SelectAll().Where(
		models.SettingsItems.TenantId.Eq(tenantID),
		models.SettingsItems.DeletedAt.IsNull(),
	))
	if err != nil {
		return nil, nil, nil, err
	}

	var (
		netName, netID, cidr, imageID string
		zoneEnum                      deployment.Yandex_Zone
		platformEnum                  deployment.Yandex_PlatformId
		accel                         deployment.Yandex_NetworkAcceleration
		publicIP                      bool
	)
	for _, it := range items {
		v := it.GetValue()
		switch it.GetKey() {
		case models.SettingsItem_KEY_YANDEX_CLOUD_NETWORK_ID:
			netID = v.GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_NETWORK_NAME:
			netName = v.GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_SUBNET_CIDR:
			cidr = v.GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_IMAGE_ID:
			imageID = v.GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_ASSIGN_PUBLIC_IP:
			publicIP = v.GetBoolValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_SOFTWARE_ACCELERATED_NETWORK:
			if v.GetBoolValue() {
				accel = deployment.Yandex_NETWORK_ACCELERATION_SOFTWARE_ACCELERATED
			}
		case models.SettingsItem_KEY_YANDEX_CLOUD_PLATFORM_ID:
			platformEnum, err = mapPlatform(v.GetYandexCloudPlatformId())
			if err != nil {
				return nil, nil, nil, err
			}
		case models.SettingsItem_KEY_YANDEX_CLOUD_ZONE:
			zoneEnum = mapZone(v.GetYandexCloudZone())
		}
	}

	zone := yandex.ZoneString(zoneEnum)
	network := &deployment.Yandex_Network{Name: netName, NetworkId: netID, Cidr: cidr, Zone: zone}
	compute := &deployment.Yandex_Compute{PlatformId: yandex.PlatformIDString(platformEnum), ImageId: imageID}
	vmDefaults := &deployment.Yandex_Vm{
		Zone:                zone,
		BootDiskType:        yandex.DiskTypeString(deployment.Yandex_DISK_TYPE_NETWORK_SSD),
		NetworkAcceleration: yandex.NetworkAccelerationString(accel),
		PublicIp:            publicIP,
	}
	return network, compute, vmDefaults, nil
}

// mapPlatform maps the settings platform enum to the deployment platform enum.
// Platforms the Yandex terraform module does not support (e.g. V4A/AMD) are a
// configuration error rather than a silent fallback.
func mapPlatform(p models.YandexCloudPlatformId) (deployment.Yandex_PlatformId, error) {
	switch p {
	case models.YandexCloudPlatformId_YANDEX_CLOUD_PLATFORM_ID_UNSPECIFIED:
		return deployment.Yandex_PLATFORM_ID_UNSPECIFIED, nil
	case models.YandexCloudPlatformId_YANDEX_CLOUD_PLATFORM_ID_STANDARD_V1:
		return deployment.Yandex_PLATFORM_ID_STANDARD_V1, nil
	case models.YandexCloudPlatformId_YANDEX_CLOUD_PLATFORM_ID_STANDARD_V2:
		return deployment.Yandex_PLATFORM_ID_STANDARD_V2, nil
	case models.YandexCloudPlatformId_YANDEX_CLOUD_PLATFORM_ID_STANDARD_V3:
		return deployment.Yandex_PLATFORM_ID_STANDARD_V3, nil
	case models.YandexCloudPlatformId_YANDEX_CLOUD_PLATFORM_ID_HIGHFREQ_V3:
		return deployment.Yandex_PLATFORM_ID_HIGHFREQ_V3, nil
	default:
		return deployment.Yandex_PLATFORM_ID_UNSPECIFIED,
			fmt.Errorf("provider: platform %s is not supported by the yandex terraform module", p)
	}
}

// mapZone maps the settings zone enum to the deployment zone enum.
func mapZone(z models.YandexCloudZone) deployment.Yandex_Zone {
	switch z {
	case models.YandexCloudZone_YANDEX_CLOUD_ZONE_RU_CENTRAL1_A:
		return deployment.Yandex_ZONE_RU_CENTRAL1_A
	case models.YandexCloudZone_YANDEX_CLOUD_ZONE_RU_CENTRAL1_B:
		return deployment.Yandex_ZONE_RU_CENTRAL1_B
	case models.YandexCloudZone_YANDEX_CLOUD_ZONE_RU_CENTRAL1_D:
		return deployment.Yandex_ZONE_RU_CENTRAL1_D
	default:
		return deployment.Yandex_ZONE_UNSPECIFIED
	}
}
