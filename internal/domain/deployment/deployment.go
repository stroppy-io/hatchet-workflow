package deployment

import (
	"context"
	"fmt"
	"sync"

	"github.com/stroppy-io/hatchet-workflow/internal/domain/deployment/docker"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/deployment/yandex"
	"github.com/stroppy-io/hatchet-workflow/internal/infrastructure/terraform"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
)

type Service interface {
	CreateDeployment(
		ctx context.Context,
		depl *deployment.Deployment_Template,
	) (*deployment.Deployment, error)
	DestroyDeployment(
		ctx context.Context,
		depl *deployment.Deployment,
	) error
}

type Registry map[deployment.Target]Service

type TerraformActor interface {
	ApplyTerraform(
		ctx context.Context,
		params *terraform.WorkdirWithParams,
	) (terraform.TfOutput, error)
	DestroyTerraform(ctx context.Context, wd terraform.WdId) error
}

var (
	terraformActor     TerraformActor
	onceTerraformActor sync.Once
)

func NewRegistry(settings *settings.Settings) (Registry, error) {
	dockerService, err := docker.NewService(settings)
	if err != nil {
		return nil, err
	}
	if settings.GetPreferredTarget() == deployment.Target_TARGET_DOCKER {
		return Registry{
			deployment.Target_TARGET_DOCKER: dockerService,
		}, nil
	}
	// TODO: Think about setup terraform only if we need it
	onceTerraformActor.Do(func() {
		terraformActor, err = terraform.NewActor()
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Terraform actor: %s", err)
	}
	return Registry{
		deployment.Target_TARGET_DOCKER:       dockerService,
		deployment.Target_TARGET_YANDEX_CLOUD: yandex.NewTerraformDeploymentService(terraformActor, settings),
	}, nil
}

var ErrUnsupportedDeploymentTarget = fmt.Errorf("unsupported deployment target")

func (r Registry) CreateDeployment(ctx context.Context, depl *deployment.Deployment_Template) (*deployment.Deployment, error) {
	svc, ok := r[depl.GetIdentifier().GetTarget()]
	if !ok {
		return nil, ErrUnsupportedDeploymentTarget
	}
	return svc.CreateDeployment(ctx, depl)
}

func (r Registry) DestroyDeployment(ctx context.Context, depl *deployment.Deployment) error {
	svc, ok := r[depl.GetTemplate().GetIdentifier().GetTarget()]
	if !ok {
		return ErrUnsupportedDeploymentTarget
	}
	return svc.DestroyDeployment(ctx, depl)
}
