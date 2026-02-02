package provision

import (
	"context"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

type QuotaManager interface {
	ReserveQuotas(ctx context.Context, quotas []*crossplane.Quota) error
}

type Config struct {
}

type Provisioner struct {
	*hatchet.UnimplementedProvisionerServer
}

func New() *Provisioner {
	return &Provisioner{
		UnimplementedProvisionerServer: &hatchet.UnimplementedProvisionerServer{},
	}
}

func (p Provisioner) ProvisionCloud(ctx context.Context, request *hatchet.ProvisionCloudRequest) (*hatchet.ProvisionCloudResponse, error) {
	runningDepl
	for _, deployment := range request.GetDeployments() {

	}
	return &hatchet.ProvisionCloudResponse{
		RunId:       request.GetRunId(),
		Deployments: request.GetDeployments(),
	}, nil
}

func (p Provisioner) RemoveCloud(ctx context.Context, request *hatchet.ProvisionCloudRequest) (*hatchet.ProvisionCloudResponse, error) {
	//TODO implement me
	panic("implement me")
}
