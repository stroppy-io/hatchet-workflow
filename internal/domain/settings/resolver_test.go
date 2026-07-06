package settings

import (
	"context"
	"testing"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	apipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	modelspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestResolverReadsProviderAndAgentSettingsThroughInterfaces(t *testing.T) {
	resolver := Resolver{
		PlatformSource: StaticPlatformSettingsSource{
			Settings: &apipb.PlatformSettings{ServerAddr: "https://control.example"},
		},
		TenantSource: StaticTenantSettingsSource{
			Settings: map[string]*modelspb.TenantSettingsRecord{
				"tenant-1": {
					YandexSettings: &deploymentpb.Yandex_Settings{
						Token:        "token",
						CloudId:      "cloud-id",
						FolderId:     "folder-id",
						Zone:         deploymentpb.Yandex_Settings_ZONE_RU_CENTRAL1_A,
						NetworkId:    "network-id",
						NetworkName:  "stroppy",
						SubnetCidr:   "10.0.0.0/8",
						PlatformId:   deploymentpb.Yandex_Settings_PLATFORM_ID_STANDARD_V2,
						ImageId:      "image-id",
						SshUser:      "stroppy",
						SshPublicKey: "ssh-rsa test",
					},
				},
			},
		},
		DefaultServerAddr:        "https://env.example",
		DefaultTemporalNamespace: "bench",
		DefaultAgentEnv:          map[string]string{"CUSTOM_ENV": "value"},
	}

	providerSettings, err := resolver.ProviderSettings(context.Background(), "tenant-1", deploymentpb.Provider_PROVIDER_YANDEX)
	if err != nil {
		t.Fatalf("provider settings: %v", err)
	}
	if got, want := providerSettings.GetYandex().GetCloudId(), "cloud-id"; got != want {
		t.Fatalf("cloud id = %q, want %q", got, want)
	}

	bootstrap, err := resolver.AgentBootstrap(context.Background())
	if err != nil {
		t.Fatalf("agent bootstrap: %v", err)
	}
	if got, want := bootstrap.GetServerAddr(), "https://env.example"; got != want {
		t.Fatalf("server addr = %q, want %q", got, want)
	}
	if got, want := bootstrap.GetTemporalNamespace(), "bench"; got != want {
		t.Fatalf("temporal namespace = %q, want %q", got, want)
	}
	if got := bootstrap.GetBinaryUrl(); got != "" {
		t.Fatalf("binary url = %q, want empty so agent derives it from server addr", got)
	}
	if got, want := bootstrap.GetExtraEnv()["CUSTOM_ENV"], "value"; got != want {
		t.Fatalf("extra env = %q, want %q", got, want)
	}
}

func TestResolverFallsBackToPlatformServerAddressWhenNoRuntimeDefault(t *testing.T) {
	resolver := Resolver{
		PlatformSource: StaticPlatformSettingsSource{
			Settings: &apipb.PlatformSettings{ServerAddr: "https://control.example"},
		},
	}

	bootstrap, err := resolver.AgentBootstrap(context.Background())
	if err != nil {
		t.Fatalf("agent bootstrap: %v", err)
	}
	if got, want := bootstrap.GetServerAddr(), "https://control.example"; got != want {
		t.Fatalf("server addr = %q, want %q", got, want)
	}
}

func TestAgentBootstrapInjectsRegistryMirrorEnv(t *testing.T) {
	resolver := Resolver{
		DefaultServerAddr: "https://env.example",
	}

	bootstrap, err := resolver.AgentBootstrap(context.Background())
	if err != nil {
		t.Fatalf("agent bootstrap: %v", err)
	}

	wantMirror := agentdomain.AptProxyURL("https://env.example")
	if got := bootstrap.GetExtraEnv()[registryMirrorEnv]; got != wantMirror {
		t.Fatalf("ExtraEnv[%q] = %q, want %q", registryMirrorEnv, got, wantMirror)
	}
}

func TestAgentBootstrapRegistryMirrorEnvOverrideWins(t *testing.T) {
	overrideURL := "http://custom-registry:5000"
	resolver := Resolver{
		DefaultServerAddr: "https://env.example",
		DefaultAgentEnv:   map[string]string{registryMirrorEnv: overrideURL},
	}

	bootstrap, err := resolver.AgentBootstrap(context.Background())
	if err != nil {
		t.Fatalf("agent bootstrap: %v", err)
	}

	if got := bootstrap.GetExtraEnv()[registryMirrorEnv]; got != overrideURL {
		t.Fatalf("ExtraEnv[%q] = %q, want explicit override %q", registryMirrorEnv, got, overrideURL)
	}
}

func TestResolverDefaultsDockerProviderAndLocalAgentAddress(t *testing.T) {
	resolver := Resolver{}

	providerSettings, err := resolver.ProviderSettings(context.Background(), "tenant-1", deploymentpb.Provider_PROVIDER_DOCKER)
	if err != nil {
		t.Fatalf("provider settings: %v", err)
	}
	if providerSettings.GetDocker() == nil {
		t.Fatal("docker settings are missing")
	}

	bootstrap, err := resolver.AgentBootstrap(context.Background())
	if err != nil {
		t.Fatalf("agent bootstrap: %v", err)
	}
	if got, want := bootstrap.GetServerAddr(), DefaultLocalServerAddr; got != want {
		t.Fatalf("server addr = %q, want %q", got, want)
	}
}
