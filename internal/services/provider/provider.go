// Package provider resolves a tenant's provider settings (SettingsService) into
// dag.DeploymentParams — the terraform variables the dag compiler needs. Settings
// enums (platform/zone) are translated to the terraform string forms here.
package provider

import (
	"context"
	"fmt"

	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	dagdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
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

// Resolve builds DeploymentParams from the tenant's Yandex Cloud settings.
//
// TODO(provider): per-machine internal_ip (subnet network allocation, D20) and
// user_data (agent cloud-init JWT, D18) are left empty — wire them when those land.
func (r *Resolver) Resolve(ctx context.Context, tenantID string) (*dagdomain.DeploymentParams, error) {
	items, err := r.items.Query(ctx, models.SettingsItems.SelectAll().Where(
		models.SettingsItems.TenantId.Eq(tenantID),
		models.SettingsItems.DeletedAt.IsNull(),
	))
	if err != nil {
		return nil, err
	}
	p := &dagdomain.DeploymentParams{
		MachineInternalIP: map[string]string{},
		MachineUserData:   map[string]string{},
	}
	for _, it := range items {
		v := it.GetValue()
		switch it.GetKey() {
		case models.SettingsItem_KEY_YANDEX_CLOUD_NETWORK_ID:
			p.NetworkID = v.GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_NETWORK_NAME:
			p.NetworkName = v.GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_SUBNET_CIDR:
			p.NetworkCIDR = v.GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_IMAGE_ID:
			p.ImageID = v.GetStringValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_ASSIGN_PUBLIC_IP:
			p.AssignPublicIP = v.GetBoolValue()
		case models.SettingsItem_KEY_YANDEX_CLOUD_SOFTWARE_ACCELERATED_NETWORK:
			if v.GetBoolValue() {
				p.NetworkAcceleration = deployment.Yandex_NETWORK_ACCELERATION_SOFTWARE_ACCELERATED
			}
		case models.SettingsItem_KEY_YANDEX_CLOUD_PLATFORM_ID:
			platform, err := mapPlatform(v.GetYandexCloudPlatformId())
			if err != nil {
				return nil, err
			}
			p.Platform = platform
		case models.SettingsItem_KEY_YANDEX_CLOUD_ZONE:
			p.Zone = mapZone(v.GetYandexCloudZone())
		}
	}
	return p, nil
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
