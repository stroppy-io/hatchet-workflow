package quotas

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

type TenantSettingsReader interface {
	Get(ctx context.Context, tenantID string) (*models.TenantSettingsRecord, error)
}

type ManagerConfig struct {
	SnapshotTTL    time.Duration
	ReservationTTL time.Duration
	Now            func() time.Time
}

type Manager struct {
	store    *Store
	settings TenantSettingsReader
	sources  map[deploymentpb.Provider]ProviderSource
	cfg      ManagerConfig
}

func NewManager(store *Store, settings TenantSettingsReader, sources map[deploymentpb.Provider]ProviderSource, cfg ManagerConfig) *Manager {
	if cfg.SnapshotTTL <= 0 {
		cfg.SnapshotTTL = 5 * time.Minute
	}
	if cfg.ReservationTTL <= 0 {
		cfg.ReservationTTL = 30 * time.Minute
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if sources == nil {
		sources = map[deploymentpb.Provider]ProviderSource{}
	}
	return &Manager{store: store, settings: settings, sources: sources, cfg: cfg}
}

func (m *Manager) ListQuotas(ctx context.Context, tenantID string, provider deploymentpb.Provider, policy api.QuotaRefreshPolicy) ([]*api.QuotaView, error) {
	if tenantID == "" {
		return nil, derrors.Invalid("tenant_id", "tenant_id is required")
	}
	now := m.now()
	if policy == api.QuotaRefreshPolicy_QUOTA_REFRESH_POLICY_UNSPECIFIED {
		policy = api.QuotaRefreshPolicy_QUOTA_REFRESH_POLICY_REFRESH_IF_STALE
	}
	if policy != api.QuotaRefreshPolicy_QUOTA_REFRESH_POLICY_CACHE_ONLY {
		if provider == deploymentpb.Provider_PROVIDER_UNSPECIFIED {
			if err := m.refreshConfigured(ctx, tenantID, policy); err != nil {
				return nil, err
			}
		} else {
			scope, settings, err := m.scopeForProvider(ctx, tenantID, provider, nil)
			if err != nil {
				return nil, err
			}
			needsRefresh := policy == api.QuotaRefreshPolicy_QUOTA_REFRESH_POLICY_FORCE_REFRESH
			if !needsRefresh {
				needsRefresh, err = m.store.ScopeNeedsRefresh(ctx, scope, nil, now)
				if err != nil {
					return nil, err
				}
			}
			if needsRefresh {
				if err := m.refreshScope(ctx, scope, settings, nil); err != nil {
					return nil, err
				}
			}
		}
	}
	return m.store.ListQuotaViews(ctx, tenantID, provider, m.now())
}

func (m *Manager) RefreshQuotas(ctx context.Context, tenantID string, provider deploymentpb.Provider) ([]*api.QuotaView, error) {
	if provider == deploymentpb.Provider_PROVIDER_UNSPECIFIED {
		return nil, derrors.Invalid("provider", "provider is required")
	}
	scope, settings, err := m.scopeForProvider(ctx, tenantID, provider, nil)
	if err != nil {
		return nil, err
	}
	if err := m.refreshScope(ctx, scope, settings, nil); err != nil {
		return nil, err
	}
	return m.store.ListQuotaViews(ctx, tenantID, provider, m.now())
}

func (m *Manager) RunUsage(ctx context.Context, tenantID, runID string) ([]*api.QuotaReservationView, error) {
	if tenantID == "" {
		return nil, derrors.Invalid("tenant_id", "tenant_id is required")
	}
	if runID == "" {
		return nil, derrors.Invalid("run_id", "run_id is required")
	}
	reservations, err := m.store.ListRunReservations(ctx, tenantID, runID)
	if err != nil {
		return nil, err
	}
	return ReservationsToViews(reservations), nil
}

func (m *Manager) RefreshAllConfigured(ctx context.Context) error {
	tenantIDs, err := m.store.ListYandexTenantIDs(ctx)
	if err != nil {
		return err
	}
	for _, tenantID := range tenantIDs {
		if err := m.refreshConfigured(ctx, tenantID, api.QuotaRefreshPolicy_QUOTA_REFRESH_POLICY_FORCE_REFRESH); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) refreshConfigured(ctx context.Context, tenantID string, policy api.QuotaRefreshPolicy) error {
	now := m.now()
	for _, provider := range []deploymentpb.Provider{deploymentpb.Provider_PROVIDER_YANDEX, deploymentpb.Provider_PROVIDER_DOCKER} {
		scope, settings, err := m.scopeForProvider(ctx, tenantID, provider, nil)
		if errors.Is(err, derrors.ErrNotFound) || errors.Is(err, ErrProviderNotConfigured) {
			continue
		}
		if err != nil {
			return err
		}
		needsRefresh := policy == api.QuotaRefreshPolicy_QUOTA_REFRESH_POLICY_FORCE_REFRESH
		if !needsRefresh {
			needsRefresh, err = m.store.ScopeNeedsRefresh(ctx, scope, nil, now)
			if err != nil {
				return err
			}
		}
		if needsRefresh {
			if err := m.refreshScope(ctx, scope, settings, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) refreshScope(ctx context.Context, scope Scope, settings *deploymentpb.ProviderSettings, services []string) error {
	source := m.sources[scope.Provider]
	if source == nil {
		return derrors.FailedPrecondition("QUOTA_PROVIDER_UNSUPPORTED", fmt.Sprintf("quota provider %s is not configured", scope.Provider))
	}
	snapshots, err := source.ListQuotas(ctx, SourceRequest{
		TenantID: scope.TenantID,
		Scope:    scope,
		Settings: settings,
		Services: services,
	})
	if err != nil {
		return err
	}
	now := m.now()
	for i := range snapshots {
		snapshots[i].TenantID = scope.TenantID
		snapshots[i].Provider = scope.Provider
		if snapshots[i].ResourceType == "" {
			snapshots[i].ResourceType = scope.ResourceType
		}
		if snapshots[i].ResourceID == "" {
			snapshots[i].ResourceID = scope.ResourceID
		}
		if snapshots[i].ObservedAt.IsZero() {
			snapshots[i].ObservedAt = now
		}
		if snapshots[i].StaleAfter.IsZero() {
			snapshots[i].StaleAfter = snapshots[i].ObservedAt.Add(m.cfg.SnapshotTTL)
		}
		if snapshots[i].ProviderAvailable == 0 && snapshots[i].Limit > 0 {
			snapshots[i].ProviderAvailable = math.Max(snapshots[i].Limit-snapshots[i].ProviderUsed, 0)
		}
	}
	return m.store.UpsertSnapshots(ctx, snapshots)
}

var ErrProviderNotConfigured = errors.New("quota provider not configured")

func (m *Manager) scopeForProvider(ctx context.Context, tenantID string, provider deploymentpb.Provider, planSettings *deploymentpb.ProviderSettings) (Scope, *deploymentpb.ProviderSettings, error) {
	switch provider {
	case deploymentpb.Provider_PROVIDER_DOCKER:
		return Scope{TenantID: tenantID, Provider: provider, ResourceType: ResourceTypeDockerHost, ResourceID: "local", Service: "docker"}, &deploymentpb.ProviderSettings{
			Settings: &deploymentpb.ProviderSettings_Docker{Docker: &deploymentpb.Docker_Settings{}},
		}, nil
	case deploymentpb.Provider_PROVIDER_YANDEX:
		yandexSettings, err := m.yandexSettings(ctx, tenantID, planSettings)
		if err != nil {
			return Scope{}, nil, err
		}
		settings := &deploymentpb.ProviderSettings{
			Settings: &deploymentpb.ProviderSettings_Yandex{Yandex: yandexSettings},
		}
		return Scope{
			TenantID:     tenantID,
			Provider:     provider,
			ResourceType: ResourceTypeYandexCloud,
			ResourceID:   yandexSettings.GetCloudId(),
			Service:      "",
		}, settings, nil
	default:
		return Scope{}, nil, derrors.FailedPrecondition("QUOTA_PROVIDER_UNSUPPORTED", fmt.Sprintf("quota provider %s is not supported", provider))
	}
}

func (m *Manager) yandexSettings(ctx context.Context, tenantID string, planSettings *deploymentpb.ProviderSettings) (*deploymentpb.Yandex_Settings, error) {
	var base *deploymentpb.Yandex_Settings
	if m.settings != nil {
		rec, err := m.settings.Get(ctx, tenantID)
		if err != nil && !errors.Is(err, derrors.ErrNotFound) {
			return nil, err
		}
		if rec != nil && rec.GetYandexSettings() != nil {
			base = proto.Clone(rec.GetYandexSettings()).(*deploymentpb.Yandex_Settings)
		}
	}
	if base == nil {
		base = &deploymentpb.Yandex_Settings{}
	}
	if plan := planSettings.GetYandex(); plan != nil {
		overlayYandexSettings(base, plan)
	}
	if base.GetCloudId() == "" || base.GetToken() == "" {
		return nil, derrors.FailedPrecondition("QUOTA_PROVIDER_NOT_CONFIGURED", "yandex quota provider is not configured").Wrap(ErrProviderNotConfigured)
	}
	return base, nil
}

func overlayYandexSettings(base, override *deploymentpb.Yandex_Settings) {
	if override.GetToken() != "" {
		base.Token = override.GetToken()
	}
	if override.GetCloudId() != "" {
		base.CloudId = override.GetCloudId()
	}
	if override.GetFolderId() != "" {
		base.FolderId = override.GetFolderId()
	}
	if override.GetZone() != deploymentpb.Yandex_Settings_ZONE_UNSPECIFIED {
		base.Zone = override.GetZone()
	}
	if override.GetNetworkId() != "" {
		base.NetworkId = override.GetNetworkId()
	}
	if override.GetNetworkName() != "" {
		base.NetworkName = override.GetNetworkName()
	}
	if override.GetSubnetCidr() != "" {
		base.SubnetCidr = override.GetSubnetCidr()
	}
	if override.GetPlatformId() != deploymentpb.Yandex_Settings_PLATFORM_ID_UNSPECIFIED {
		base.PlatformId = override.GetPlatformId()
	}
	if override.GetImageId() != "" {
		base.ImageId = override.GetImageId()
	}
	if override.GetAssignPublicIp() {
		base.AssignPublicIp = true
	}
	if override.GetSoftwareAcceleratedNetwork() {
		base.SoftwareAcceleratedNetwork = true
	}
	if override.GetSshUser() != "" {
		base.SshUser = override.GetSshUser()
	}
	if override.GetSshPublicKey() != "" {
		base.SshPublicKey = override.GetSshPublicKey()
	}
}

func (m *Manager) now() time.Time { return m.cfg.Now().UTC() }

func unitsOrInfer(units, quotaName string) string {
	if units != "" {
		return units
	}
	switch {
	case strings.HasSuffix(quotaName, ".count"):
		return "count"
	case strings.HasSuffix(quotaName, ".size"):
		return "GiB"
	case strings.HasSuffix(quotaName, ".rate"):
		return "rate"
	default:
		return "count"
	}
}

