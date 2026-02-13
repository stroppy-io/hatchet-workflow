package yandex

import (
	"context"
	"embed"
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/core/utils"
	"github.com/stroppy-io/hatchet-workflow/internal/infrastructure/terraform"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
)

const (
	YcTokenEnvKey    consts.EnvKey = "YC_TOKEN"
	YcCloudIdEnvKey  consts.EnvKey = "YC_CLOUD_ID"
	YcFolderIdEnvKey consts.EnvKey = "YC_FOLDER_ID"
	YcZoneEnvKey     consts.EnvKey = "YC_ZONE"
)

type TerraformActor interface {
	ApplyTerraform(
		ctx context.Context,
		wd terraform.WdId,
		tfFiles []terraform.TfFile,
		varFile terraform.TfVarFile,
		env terraform.TfEnv,
	) (terraform.TfOutput, error)
	DestroyTerraform(ctx context.Context, wd terraform.WdId) error
}

//go:embed *.tf
var tfFilesEmbed embed.FS

func files() ([]terraform.TfFile, error) {
	filenames, err := utils.GetAllEmbedFilenames(&tfFilesEmbed, ".")
	if err != nil {
		return nil, err
	}
	var out []terraform.TfFile
	for _, filename := range filenames {
		content, err := tfFilesEmbed.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		out = append(out, terraform.NewTfFile(content, filename))
	}
	return out, nil

}

type TerraformDeploymentService struct {
	actor    TerraformActor
	settings *settings.Settings
}

func NewTerraformDeploymentService(
	actor TerraformActor,
	settings *settings.Settings,
) *TerraformDeploymentService {
	return &TerraformDeploymentService{
		actor:    actor,
		settings: settings,
	}
}

func (s *TerraformDeploymentService) CreateDeployment(
	ctx context.Context,
	depl *deployment.Deployment_Template,
) (*deployment.Deployment, error) {
	if err := depl.Validate(); err != nil {
		return nil, fmt.Errorf("error validating deployment template: %s", err)
	}
	tfFiles, err := files()
	if err != nil {
		return nil, err
	}
	vars, err := terraform.NewTfVarFile(VariablesFromTemplate(s.settings, depl))
	if err != nil {
		return nil, err
	}
	output, err := s.actor.ApplyTerraform(
		ctx,
		terraform.NewWdId(depl.GetIdentifier().GetId()),
		tfFiles,
		vars,
		map[string]string{
			YcTokenEnvKey:    s.settings.GetYandexCloud().GetProviderSettings().GetToken(),
			YcCloudIdEnvKey:  s.settings.GetYandexCloud().GetProviderSettings().GetCloudId(),
			YcFolderIdEnvKey: s.settings.GetYandexCloud().GetProviderSettings().GetFolderId(),
			YcZoneEnvKey:     s.settings.GetYandexCloud().GetProviderSettings().GetZone(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error applying Terraform: %s", err)
	}
	vmIps, err := GetVmIpsOutput(output)
	if err != nil {
		return nil, fmt.Errorf("error getting vm ips output: %s", err)
	}

	createVms := make([]*deployment.Vm, 0)
	for _, vmTmpl := range depl.GetVmTemplates() {
		createVms = append(createVms, &deployment.Vm{
			Template:           vmTmpl,
			AssignedInternalIp: &deployment.Ip{Value: vmIps[vmTmpl.GetIdentifier().GetName()].InternalIP},
			AssignedExternalIp: &deployment.Ip{Value: vmIps[vmTmpl.GetIdentifier().GetName()].NatIP},
		})
	}

	return &deployment.Deployment{
		Template: depl,
		Vms:      createVms,
	}, nil
}

func (s *TerraformDeploymentService) DestroyDeployment(
	ctx context.Context,
	depl *deployment.Deployment,
) error {
	return s.actor.DestroyTerraform(ctx, terraform.NewWdId(depl.GetTemplate().GetIdentifier().GetId()))
}
