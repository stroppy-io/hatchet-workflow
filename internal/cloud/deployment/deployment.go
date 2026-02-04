package deployment

import (
	"fmt"

	"github.com/oklog/ulid/v2"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

type DetailsBuilder interface {
	BuildVmCloudDetails(
		id string,
		network *crossplane.Network,
		vm *crossplane.Deployment_Vm,
	) (*crossplane.Deployment_CloudDetails, error)
}

type Builder struct {
	mapping map[crossplane.SupportedCloud]DetailsBuilder
}

func NewBuilder(mapping map[crossplane.SupportedCloud]DetailsBuilder) *Builder {
	return &Builder{
		mapping: mapping,
	}
}

func (b *Builder) BuildDeploymentDetails(
	cloud crossplane.SupportedCloud,
	id string,
	network *crossplane.Network,
	vm *crossplane.Deployment_Vm,
) (*crossplane.Deployment_CloudDetails, error) {
	if builder, ok := b.mapping[cloud]; ok {
		return builder.BuildVmCloudDetails(id, network, vm)
	}
	return nil, fmt.Errorf("unsupported deployment type")
}

type detailsBuilder interface {
	BuildDeploymentDetails(
		cloud crossplane.SupportedCloud,
		id string,
		network *crossplane.Network,
		vm *crossplane.Deployment_Vm,
	) (*crossplane.Deployment_CloudDetails, error)
}

func NewDeploymentFromRequest(
	network *crossplane.Network,
	request *crossplane.Deployment_Request,
	builder detailsBuilder,
) (*crossplane.Deployment, error) {
	err := request.Validate()
	if err != nil {
		return nil, err
	}
	deploymentId := ulid.Make().String()
	vm := &crossplane.Deployment_Vm{
		MachineInfo: request.GetMachineInfo(),
		CloudInit:   request.GetCloudInit(),
	}
	cloudDetails, err := builder.BuildDeploymentDetails(
		request.GetSupportedCloud(),
		deploymentId,
		network,
		vm,
	)
	if err != nil {
		return nil, err
	}
	template := &crossplane.Deployment{
		Id:             deploymentId,
		Name:           request.GetName(),
		SupportedCloud: request.GetSupportedCloud(),
		Vm:             vm,
		CloudDetails:   cloudDetails,
		Labels:         request.GetLabels(),
	}
	template.CloudDetails = cloudDetails
	return template, nil
}

func NewDeploymentSetFromRequest(
	network *crossplane.Network,
	request *crossplane.DeploymentSet_Request,
	builder detailsBuilder,
) (*crossplane.DeploymentSet, error) {
	counts := make(map[crossplane.SupportedCloud]int)
	for _, req := range request.GetRequests() {
		counts[req.GetSupportedCloud()]++
	}
	if len(counts) != 1 {
		return nil, fmt.Errorf("cannot create deployment set with multiple cloud types")
	}
	err := request.Validate()
	if err != nil {
		return nil, err
	}
	set := &crossplane.DeploymentSet{
		Network:     network,
		Deployments: make([]*crossplane.Deployment, 0, len(request.GetRequests())),
	}
	for _, req := range request.GetRequests() {
		deployment, err := NewDeploymentFromRequest(network, req, builder)
		if err != nil {
			return nil, err
		}
		set.Deployments = append(set.Deployments, deployment)
	}
	return set, nil
}
