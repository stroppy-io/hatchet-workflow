package provision

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/provider/docker"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/provider/yandex"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/terraform"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/managers"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	valkeygo "github.com/valkey-io/valkey-go"
)

const (
	K8SConfigPath = "K8S_CONFIG_PATH"
	ValkeyUrl     = "VALKEY_URL"
	YcToken       = "YC_TOKEN"
)

type DeploymentSvcMap map[crossplane.SupportedCloud]deployment.DeploymentService

type Deps struct {
	QuotaManager   *managers.QuotaManager
	NetworkManager *managers.NetworkManager
	Factory        *deployment.Factory
	DeploySvcMap   DeploymentSvcMap
}

func (d *Deps) GetDeploymentSvc(cloud crossplane.SupportedCloud) (deployment.DeploymentService, error) {
	svc, ok := d.DeploySvcMap[cloud]
	if !ok {
		return nil, fmt.Errorf("no deployment service registered for cloud: %s", cloud)
	}
	return svc, nil
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
	valkeyClient, err := valkeyFromEnv()
	if err != nil {
		return nil, err
	}
	networkManager, err := managers.NewNetworkManager(valkeyClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create network manager: %w", err)
	}

	builders := deployment.BuildersMap{}
	svcMap := DeploymentSvcMap{}
	allQuotas := &managers.QuotasConfig{}

	// Register Yandex Cloud provider (Terraform based)
	// We check for YC_TOKEN to enable Yandex Cloud support
	if os.Getenv(YcToken) != "" {
		builders[crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX] = yandex.NewTerraformBuilder(&yandex.ProviderConfig{
			K8sNamespace:        DefaultCrossplaneNamespace, // Still used for naming/namespaces in internal logic if needed
			DefaultVmZone:       DefaultVmZone,
			DefaultVmPlatformId: DefaultVmPlatformId,
		})

		// Use a dedicated directory for Terraform state and configs
		terraformDir := os.Getenv("TERRAFORM_DIR")
		if terraformDir == "" {
			terraformDir = "/tmp/hatchet-terraform"
		}

		svcMap[crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX] = terraform.NewService(terraformDir)
		allQuotas.AvailableQuotas = append(allQuotas.AvailableQuotas, managers.DefaultQuotasConfig().AvailableQuotas...)
	}

	// Register Docker provider (always available)
	dockerCfg := docker.ProviderConfigFromEnv()
	dockerSvc, err := docker.NewService(dockerCfg.NetworkName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker service: %w", err)
	}
	builders[crossplane.SupportedCloud_SUPPORTED_CLOUD_DOCKER] = docker.NewCloudBuilder(dockerCfg)
	svcMap[crossplane.SupportedCloud_SUPPORTED_CLOUD_DOCKER] = dockerSvc
	allQuotas.AvailableQuotas = append(allQuotas.AvailableQuotas, dockerQuotasConfig().AvailableQuotas...)

	quotaManager, err := managers.NewQuotaManager(valkeyClient, allQuotas)
	if err != nil {
		return nil, fmt.Errorf("failed to create quota manager: %w", err)
	}

	return &Deps{
		QuotaManager:   quotaManager,
		NetworkManager: networkManager,
		Factory:        deployment.NewDeploymentFactory(builders),
		DeploySvcMap:   svcMap,
	}, nil
}

func dockerQuotasConfig() *managers.QuotasConfig {
	return &managers.QuotasConfig{
		AvailableQuotas: []*crossplane.Quota{
			{
				Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_DOCKER,
				Kind:    crossplane.Quota_KIND_VM,
				Maximum: 100,
			},
			{
				Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_DOCKER,
				Kind:    crossplane.Quota_KIND_SUBNET,
				Maximum: 100,
			},
		},
	}
}

func (d *Deps) FallbackDestroyDeployment(
	ctx context.Context,
	cloud crossplane.SupportedCloud,
	target *crossplane.Deployment,
) error {
	return backoff.Retry(func() error {
		svc, err := d.GetDeploymentSvc(cloud)
		if err != nil {
			return backoff.Permanent(err)
		}
		if err := svc.DestroyDeployment(ctx, target); err != nil {
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
