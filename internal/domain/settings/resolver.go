package settings

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	apipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	modelspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

const DefaultLocalServerAddr = "http://host.docker.internal:8080"

// registryMirrorEnv is the env var the agent expands (from the bootstrap
// extra-env map) into its docker pull-through registry mirror config.
// Inlined here (previously lived on the per-database orioledb package) since
// the per-database engine packages were deleted in the old-backend cleanup,
// but domain/settings — a shared seam that cleanup's recon missed — still
// needs the constant to stamp every agent bootstrap's extra env.
const registryMirrorEnv = "STROPPY_REGISTRY_MIRROR"

type PlatformSettingsSource interface {
	PlatformSettings(ctx context.Context) (*apipb.PlatformSettings, error)
}

type TenantSettingsSource interface {
	TenantSettings(ctx context.Context, tenantID string) (*modelspb.TenantSettingsRecord, error)
}

type ProviderSettingsSource interface {
	ProviderSettings(ctx context.Context, tenantID string, provider deploymentpb.Provider) (*deploymentpb.ProviderSettings, error)
}

type AgentBootstrapSource interface {
	AgentBootstrap(ctx context.Context) (*workflowpb.AgentBootstrap, error)
}

type Resolver struct {
	PlatformSource PlatformSettingsSource
	TenantSource   TenantSettingsSource

	DefaultServerAddr        string
	DefaultTemporalNamespace string
	DefaultAgentEnv          map[string]string
}

func (r Resolver) ProviderSettings(ctx context.Context, tenantID string, provider deploymentpb.Provider) (*deploymentpb.ProviderSettings, error) {
	switch provider {
	case deploymentpb.Provider_PROVIDER_DOCKER:
		return &deploymentpb.ProviderSettings{
			Settings: &deploymentpb.ProviderSettings_Docker{Docker: &deploymentpb.Docker_Settings{}},
		}, nil
	case deploymentpb.Provider_PROVIDER_YANDEX:
		if r.TenantSource == nil {
			return nil, errors.New("tenant settings source is required for yandex provider")
		}
		record, err := r.TenantSource.TenantSettings(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		yandex := record.GetYandexSettings()
		if yandex == nil {
			return nil, fmt.Errorf("tenant %q has no yandex provider settings", tenantID)
		}
		clone := proto.Clone(yandex).(*deploymentpb.Yandex_Settings)
		return &deploymentpb.ProviderSettings{
			Settings: &deploymentpb.ProviderSettings_Yandex{Yandex: clone},
		}, nil
	default:
		return nil, fmt.Errorf("provider %s is not supported", provider)
	}
}

func (r Resolver) AgentBootstrap(ctx context.Context) (*workflowpb.AgentBootstrap, error) {
	serverAddr := r.DefaultServerAddr
	if serverAddr == "" && r.PlatformSource != nil {
		settings, err := r.PlatformSource.PlatformSettings(ctx)
		if err != nil {
			return nil, err
		}
		if settings.GetServerAddr() != "" {
			serverAddr = settings.GetServerAddr()
		}
	}
	if serverAddr == "" {
		serverAddr = DefaultLocalServerAddr
	}

	extraEnv := copyStringMap(r.DefaultAgentEnv)
	// Inject the pull-through registry mirror URL so agents can pull Docker
	// images through the gateway without direct internet access. Uses the same
	// host as the apt proxy (agentdomain.AptProxyURL). Skipped when serverAddr
	// is empty or the caller already provides an explicit override via
	// DefaultAgentEnv (let an explicit setting win).
	if serverAddr != "" {
		if _, alreadySet := extraEnv[registryMirrorEnv]; !alreadySet {
			// mirror URL == server origin; /v2 is routed to the registry backend by the gateway (and Caddy).
			if mirrorURL := agentdomain.AptProxyURL(serverAddr); mirrorURL != "" {
				extraEnv[registryMirrorEnv] = mirrorURL
			}
		}
	}

	bootstrap := &workflowpb.AgentBootstrap{
		ServerAddr:        serverAddr,
		TemporalNamespace: r.DefaultTemporalNamespace,
		ExtraEnv:          extraEnv,
	}
	if err := bootstrap.Validate(); err != nil {
		return nil, err
	}
	return bootstrap, nil
}

type StaticPlatformSettingsSource struct {
	Settings *apipb.PlatformSettings
}

func (s StaticPlatformSettingsSource) PlatformSettings(context.Context) (*apipb.PlatformSettings, error) {
	if s.Settings == nil {
		return &apipb.PlatformSettings{}, nil
	}
	return proto.Clone(s.Settings).(*apipb.PlatformSettings), nil
}

type StaticTenantSettingsSource struct {
	Settings map[string]*modelspb.TenantSettingsRecord
}

func (s StaticTenantSettingsSource) TenantSettings(_ context.Context, tenantID string) (*modelspb.TenantSettingsRecord, error) {
	settings, ok := s.Settings[tenantID]
	if !ok || settings == nil {
		return nil, fmt.Errorf("tenant settings for %q not found", tenantID)
	}
	return proto.Clone(settings).(*modelspb.TenantSettingsRecord), nil
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
