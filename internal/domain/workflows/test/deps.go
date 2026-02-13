package test

import (
	"github.com/stroppy-io/hatchet-workflow/internal/domain/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/managers"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
)

type Deps struct {
	ProvisionerService *provision.ProvisionerService
}

func NewDeps(settings *settings.Settings) (*Deps, error) {
	valkeyClient, err := valkeyFromEnv()
	if err != nil {
		return nil, err
	}
	networkManager, err := managers.NewNetworkManager(valkeyClient)
	if err != nil {
		return nil, err
	}
	deploymentService, err := deployment.NewRegistry(settings)
	if err != nil {
		return nil, err
	}
	provisionService := provision.NewProvisionerService(networkManager, deploymentService)
	return &Deps{
		ProvisionerService: provisionService,
	}, nil
}
