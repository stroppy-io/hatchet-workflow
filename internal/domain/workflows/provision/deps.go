package provision

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cenkalti/backoff/v4"
	crossplaneLib "github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane/k8s"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/provider/yandex"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/managers"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	valkeygo "github.com/valkey-io/valkey-go"
)

const (
	K8SConfigPath = "K8S_CONFIG_PATH"
	ValkeyUrl     = "VALKEY_URL"
)

type Deps struct {
	QuotaManager   *managers.QuotaManager
	NetworkManager *managers.NetworkManager
	Factory        *deployment.Factory
	CrossplaneSvc  *crossplaneLib.Service
}

func valkeyFromEnv() (valkeygo.Client, error) {
	urlStr := os.Getenv(ValkeyUrl)
	if urlStr == "" {
		return nil, fmt.Errorf("environment variable %s is not set", ValkeyUrl)
	}
	valkeyUrl, err := valkeygo.ParseURL(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Valkey URL: %w", err)
	}
	valkeyClient, err := valkeygo.NewClient(valkeyUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to create Valkey client: %w", err)
	}
	return valkeyClient, nil
}

func NewProvisionDeps() (*Deps, error) {
	k8sConfigPath := os.Getenv(K8SConfigPath)
	if k8sConfigPath == "" {
		return nil, fmt.Errorf("environment variable %s is not set", K8SConfigPath)
	}
	factory := deployment.NewDeploymentFactory(deployment.BuildersMap{
		crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX: yandex.NewCloudBuilder(&yandex.ProviderConfig{
			K8sNamespace:        DefaultCrossplaneNamespace,
			DefaultVmZone:       DefaultVmZone,
			DefaultVmPlatformId: DefaultVmPlatformId,
		}),
	})
	valkeyClient, err := valkeyFromEnv()
	if err != nil {
		return nil, err
	}
	networkManager, err := managers.NewNetworkManager(valkeyClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create network manager: %w", err)
	}
	quotaManager, err := managers.NewQuotaManager(valkeyClient, managers.DefaultQuotasConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create quota manager: %w", err)
	}
	k8sSvc, err := k8s.NewClient(k8sConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}
	crossplaneSvc := crossplaneLib.NewService(k8sSvc, 2*time.Minute)
	return &Deps{
		QuotaManager:   quotaManager,
		NetworkManager: networkManager,
		Factory:        factory,
		CrossplaneSvc:  crossplaneSvc,
	}, nil
}

func (d *Deps) FallbackDestroyDeployment(ctx context.Context, target *crossplane.Deployment) error {
	return backoff.Retry(func() error {
		if err := d.CrossplaneSvc.DestroyDeployment(ctx, target); err != nil {
			return err
		}
		if err := d.QuotaManager.FreeQuotas(ctx, deployment.GetDeploymentUsingQuotas(target)); err != nil {
			return err
		}
		if err := d.NetworkManager.FreeNetworkMany(
			ctx,
			target.GetTemplate().GetNetworkTemplate().GetIdentifier(),
			target.GetTemplate().GetNetworkTemplate().GetSubnets(),
		); err != nil {
			return err
		}
		return nil
	}, backoff.WithContext(
		backoff.WithMaxRetries(backoff.NewConstantBackOff(10*time.Second), 3),
		ctx,
	))
}
