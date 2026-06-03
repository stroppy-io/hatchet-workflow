package networks

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	modelspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

type TenantSettingsSource interface {
	Get(ctx context.Context, tenantID string) (*modelspb.TenantSettingsRecord, error)
}

type ManagerConfig struct {
	ReservationTTL time.Duration
	Now            func() time.Time
}

type Manager struct {
	store    *Store
	settings TenantSettingsSource
	sources  map[deploymentpb.Provider]ProviderSource
	cfg      ManagerConfig
}

func NewManager(store *Store, settings TenantSettingsSource, sources map[deploymentpb.Provider]ProviderSource, cfg ManagerConfig) *Manager {
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

func (m *Manager) Reserve(ctx context.Context, tenantID, runID, workflowID string, plan *deploymentpb.InfrastructurePlan) (string, error) {
	if tenantID == "" {
		return "", derrors.Invalid("tenant_id", "tenant_id is required")
	}
	if runID == "" {
		return "", derrors.Invalid("run_id", "run_id is required")
	}
	if plan == nil {
		return "", derrors.Invalid("plan", "infrastructure plan is required")
	}
	if plan.GetProvider() != deploymentpb.Provider_PROVIDER_YANDEX {
		return "", nil
	}
	scope, settings, err := m.scopeForProvider(ctx, tenantID, plan.GetSettings())
	if err != nil {
		return "", err
	}
	source := m.sources[deploymentpb.Provider_PROVIDER_YANDEX]
	if source == nil {
		return "", derrors.FailedPrecondition("NETWORK_PROVIDER_UNSUPPORTED", "yandex network provider is not configured")
	}
	subnets, err := source.ListSubnets(ctx, SourceRequest{
		TenantID: tenantID,
		Scope:    scope,
		Settings: settings,
	})
	if err != nil {
		return "", err
	}
	reservation, err := m.store.Reserve(ctx, ReserveInput{
		TenantID:       tenantID,
		RunID:          runID,
		WorkflowID:     workflowID,
		Scope:          scope,
		BaseCIDR:       settings.GetYandex().GetSubnetCidr(),
		ProviderCIDRs:  providerCIDRs(subnets),
		ReservationTTL: m.cfg.ReservationTTL,
		Now:            m.now(),
	})
	if err != nil {
		return "", err
	}
	return reservationCIDR(reservation), nil
}

func (m *Manager) Commit(ctx context.Context, tenantID, runID string) (string, error) {
	reservation, err := m.store.CommitRun(ctx, tenantID, runID, m.now())
	if err != nil {
		return "", err
	}
	return reservationCIDR(reservation), nil
}

func (m *Manager) Release(ctx context.Context, tenantID, runID string) (uint32, error) {
	return m.store.ReleaseRun(ctx, tenantID, runID, m.now())
}

func (m *Manager) scopeForProvider(ctx context.Context, tenantID string, planSettings *deploymentpb.ProviderSettings) (Scope, *deploymentpb.ProviderSettings, error) {
	yandexSettings, err := m.yandexSettings(ctx, tenantID, planSettings)
	if err != nil {
		return Scope{}, nil, err
	}
	settings := &deploymentpb.ProviderSettings{
		Settings: &deploymentpb.ProviderSettings_Yandex{Yandex: yandexSettings},
	}
	return Scope{
		TenantID:     tenantID,
		Provider:     deploymentpb.Provider_PROVIDER_YANDEX,
		ResourceType: ResourceTypeYandexNetwork,
		ResourceID:   yandexSettings.GetNetworkId(),
	}, settings, nil
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
	if base.GetToken() == "" || base.GetNetworkId() == "" {
		return nil, derrors.FailedPrecondition("NETWORK_PROVIDER_NOT_CONFIGURED", "yandex network provider requires token and network_id")
	}
	if base.GetSubnetCidr() == "" {
		base.SubnetCidr = defaultYandexCIDRPool
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
}

func providerCIDRs(subnets []ProviderSubnet) []string {
	total := 0
	for _, subnet := range subnets {
		total += len(subnet.CIDRs)
	}
	out := make([]string, 0, total)
	for _, subnet := range subnets {
		out = append(out, subnet.CIDRs...)
	}
	return out
}

func reservationCIDR(reservation *Reservation) string {
	if reservation == nil {
		return ""
	}
	return reservation.CIDR
}

func (m *Manager) now() time.Time { return m.cfg.Now().UTC() }
