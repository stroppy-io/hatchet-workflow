package provision

import (
	"context"
	"errors"
	"fmt"

	"github.com/sourcegraph/conc/pool"
	crossplaneLib "github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

type QuotaManager interface {
	/*
		FetchQuotas returns the current quotas for the given cloud and kind.
	*/
	ReserveQuotas(ctx context.Context, quotas []*crossplane.Quota) error
	/*
		FetchQuotas returns the current quotas for the given cloud and kind.
	*/
	FreeQuotas(ctx context.Context, quotas []*crossplane.Quota) error
}

type NetworkManager interface {
	/*
		ReserveNetwork returns a new network with the specified number of IPs.
	*/
	ReserveNetwork(ctx context.Context, ipCount int) (*crossplane.CidrWithIps, error)
	/*
		FreeNetwork releases a network back to the pool.
	*/
	FreeNetwork(ctx context.Context, network *crossplane.CidrWithIps) error
}

type DeploymentActor interface {
	/*
		CreateDeployment creates a new deployment.
	*/
	CreateDeployment(ctx context.Context, deployment *crossplane.Deployment) (*crossplane.Deployment, error)
	/*
		ProcessDeploymentStatus processes the status of a deployment.
	*/
	ProcessDeploymentStatus(ctx context.Context, deployment *crossplane.Deployment) (*crossplane.Deployment, error)
	/*
		DestroyDeployment destroys a deployment.
	*/
	DestroyDeployment(ctx context.Context, deployment *crossplane.Deployment) error
}

type DeploymentBuilder interface {
	/*
		Build builds a deployment.
	*/
	Build(runId string, deployment *crossplane.Deployment) (*crossplane.Deployment, error)
}

type DeploymentsMap = map[string]*crossplane.Deployment

const (
	PostgresVmName = "postgres"
	StroppyVmName  = "stroppy"
)

func BuildDeployments(
	ctx context.Context,
	runId string,
	quotaManager QuotaManager,
	networkManager NetworkManager,
	builder DeploymentBuilder,
	input *hatchet.NightlyCloudStroppyRequest,
) (DeploymentsMap, *crossplane.CidrWithIps, error) {
	network, err := networkManager.ReserveNetwork(ctx, 2)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to reserve network: %w", err)
	}
	postgresVm, err := builder.Build(runId, &crossplane.Deployment{
		Id:             ids.NewUlid().GetId(),
		SupportedCloud: crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
		Deployment: &crossplane.Deployment_Vm_{
			Vm: &crossplane.Deployment_Vm{
				MachineInfo: input.GetPostgresVm(),
				// TODO: CloudInit:
				NetworkParams: &crossplane.Deployment_Vm_NetworkParams{
					InternalIp:   network.GetIps()[0],
					AssignedCidr: network.GetCidr(),
					PublicIp:     false,
					ExternalIp:   nil,
				},
			},
		},
	})
	if err != nil {
		return nil, network, fmt.Errorf("failed to build postgres vm deployment: %w", err)
	}
	stroppyVm, err := builder.Build(runId, &crossplane.Deployment{
		Id:             ids.NewUlid().GetId(),
		SupportedCloud: crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
		Deployment: &crossplane.Deployment_Vm_{
			Vm: &crossplane.Deployment_Vm{
				MachineInfo: input.GetStroppyVm(),
				// TODO: CloudInit:
				NetworkParams: &crossplane.Deployment_Vm_NetworkParams{
					InternalIp:   network.GetIps()[1],
					AssignedCidr: network.GetCidr(),
					PublicIp:     false,
					ExternalIp:   nil,
				},
			},
		},
	})
	if err != nil {
		return nil, network, fmt.Errorf("failed to build stroppy vm deployment: %w", err)
	}
	quotaErr := quotaManager.ReserveQuotas(
		ctx,
		append(postgresVm.UsingQuotas, stroppyVm.UsingQuotas...),
	)
	if quotaErr != nil {
		return nil, network, fmt.Errorf("failed to reserve quotas: %w", quotaErr)
	}
	return map[string]*crossplane.Deployment{PostgresVmName: postgresVm, StroppyVmName: stroppyVm}, network, nil
}

func CreateDeployment(ctx context.Context, actor DeploymentActor, deployments DeploymentsMap) error {
	for _, deployment := range deployments {
		if _, err := actor.CreateDeployment(ctx, deployment); err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}
	}
	return nil
}

func WaitDeployment(ctx context.Context, actor DeploymentActor, deployments DeploymentsMap) error {
	waitPool := pool.New().WithContext(ctx).WithFailFast().WithCancelOnError()
	for _, deployment := range deployments {
		waitPool.Go(func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return errors.Join(ctx.Err(), fmt.Errorf(
						"timeout waiting for deployment resources ready %s",
						deployment.GetId(),
					))
				default:
					deploymentWithStatus, err := actor.ProcessDeploymentStatus(ctx, deployment)
					if err != nil {
						continue
					}
					if !crossplaneLib.IsResourcesReady(deploymentWithStatus.GetResources()) {
						continue
					}
					return nil
				}
			}
		})
	}
	return waitPool.Wait()
}

func DestroyDeployments(
	ctx context.Context,
	actor DeploymentActor,
	quotaManager QuotaManager,
	ipManager NetworkManager,
	deployments DeploymentsMap,
	network *crossplane.CidrWithIps,
) error {
	for _, deployment := range deployments {
		if err := actor.DestroyDeployment(ctx, deployment); err != nil {
			return fmt.Errorf("failed to destroy deployment: %w", err)
		}
	}
	if err := ipManager.FreeNetwork(ctx, network); err != nil {
		return fmt.Errorf("failed to free network: %w", err)
	}
	if err := quotaManager.FreeQuotas(ctx, append(
		deployments[PostgresVmName].UsingQuotas,
		deployments[StroppyVmName].UsingQuotas...,
	)); err != nil {
		return fmt.Errorf("failed to free quotas: %w", err)
	}
	return nil
}
