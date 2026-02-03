package deployment

import (
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

type YamlBuilder interface {
	BuildVmDeployment(runId string, vm *crossplane.Deployment_Vm) (*crossplane.Deployment, error)
}

type Builder struct {
	mapping map[crossplane.SupportedCloud]YamlBuilder
}

func NewBuilder(mapping map[crossplane.SupportedCloud]YamlBuilder) *Builder {
	return &Builder{
		mapping: mapping,
	}
}

func (b *Builder) Build(runId string, toBuild *crossplane.Deployment) (*crossplane.Deployment, error) {
	if builder, ok := b.mapping[toBuild.GetSupportedCloud()]; ok {
		return builder.BuildVmDeployment(runId, toBuild.GetVm())
	}
	return nil, fmt.Errorf("unsupported deployment type")
}
