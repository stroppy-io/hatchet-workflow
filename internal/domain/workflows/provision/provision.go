package provision

import (
	"errors"
	"fmt"
	"time"

	"github.com/hatchet-dev/hatchet/pkg/worker/condition"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/samber/lo"
	crossplaneLib "github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/core/uow"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

const (
	DefaultCrossplaneNamespace = "crossplane-system"
	DefaultNetworkName         = "stroppy-crossplane-net"
	DefaultSubnetName          = "stroppy-crossplane-subnet"
	//DefaultVmName              = "stroppy-crossplane-vm"
	DefaultDeploymentName   = "stroppy-crossplane-deployment"
	DefaultNetworkId        = "enp7b429s2br5pja0jci"
	DefaultUbuntuImageId    = "fd82pkek8uu0ejjkh4vn"
	DefaultVmZone           = "ru-central1-d"
	DefaultVmPlatformId     = "standard-v2"
	DefaultSubnetBaseCidr   = "10.2.0.0/16"
	DefaultSubnetBasePrefix = 24
)

const (
	CloudProvisionWorkflowName = "cloud-provision"

	LogicalProvisionTaskName = "logical-provision"
	BuildDeploymentsTaskName = "build-deployments"
	ReserveQuotasTaskName    = "reserve-quotas"
	CreateDeploymentTaskName = "create-deployments"
	WaitDeploymentTaskName   = "wait-deployments"
	WaitWorkerInHatchet      = "wait-worker-in-hatchet"
)

const (
	RunIdLableName = "run_id"
)

type FailureHandlerOutput struct {
	FailureHandled bool   `json:"failure_handled"`
	ErrorDetails   string `json:"error_details"`
	OriginalInput  string `json:"original_input"`
}

func ProvisionWorkflow(
	c *hatchetLib.Client,
) (*hatchetLib.Workflow, error) {
	deps, err := NewProvisionDeps()
	if err != nil {
		return nil, err
	}
	workflow := c.NewWorkflow(
		CloudProvisionWorkflowName,
		hatchetLib.WithWorkflowDescription("Provision Cloud Workflow"),
	)

	logicalProvisionTask := workflow.NewTask(
		LogicalProvisionTaskName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_Provision_Input,
		) (*crossplane.Deployment_Template, error) {
			err := input.Validate()
			if err != nil {
				return nil, fmt.Errorf("failed to validate workflow input: %w", err)
			}
			// NOTE: We don't support other clouds for now
			if input.GetCommon().GetSupportedCloud() != crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX {
				return nil, fmt.Errorf("unsupported cloud: %s", input.GetCommon().GetSupportedCloud())
			}
			workers := input.GetEdgeWorkersSet().GetEdgeWorkers()
			unitOfWork := uow.UnitOfWork()
			networkTemplate := &crossplane.Network_Template{
				Identifier: &crossplane.Identifier{
					// NOTE: Now we use one predefined network for all subnets
					Id:             DefaultNetworkId,
					Name:           DefaultNetworkName,
					SupportedCloud: input.GetCommon().GetSupportedCloud(),
				},
				ExternalId: DefaultNetworkId,
			}
			subnetTemplate, err := deps.NetworkManager.ReserveNetwork(
				ctx,
				networkTemplate.GetIdentifier(),
				&crossplane.Identifier{
					// NOTE: We Use one subnet per deployment for now simplifying the logic
					Id: input.GetCommon().GetRunId(),
					// NOTE: We need to create one unique subnet per deployment with k8s unique resource name
					Name:           fmt.Sprintf("%s-%s", DefaultSubnetName, input.GetCommon().GetRunId()),
					SupportedCloud: input.GetCommon().GetSupportedCloud(),
				},
				DefaultSubnetBaseCidr,
				DefaultSubnetBasePrefix,
				len(workers),
			)
			if err != nil {
				return nil, unitOfWork.Rollback(fmt.Errorf("failed to reserve network: %w", err))
			}
			// NOTE: Add rollback for network if next step fails
			unitOfWork.Add("rollback network", func() error {
				return deps.NetworkManager.FreeNetwork(ctx, networkTemplate.GetIdentifier(), subnetTemplate)
			})
			networkTemplate.Subnets = append(networkTemplate.Subnets, subnetTemplate)

			vms := make([]*crossplane.Vm_Template, len(workers))
			for idx, worker := range workers {
				vms[idx] = &crossplane.Vm_Template{
					Identifier: &crossplane.Identifier{
						Id: ids.NewUlid().Lower().String(),
						// NOTE: We need to create one unique VM per deployment with k8s unique resource name
						Name:           worker.GetWorkerName(), // Must be unique for each worker
						SupportedCloud: input.GetCommon().GetSupportedCloud(),
					},
					Hardware: worker.GetHardware(),
					// NOTE: Use predefined Ubuntu image for now simplifying the logic
					BaseImageId: DefaultUbuntuImageId,
					// NOTE: We could use internal IP from subnet cause len(subnetTemplate.GetIps()) == len(input.GetEdgeWorkersHardware())
					InternalIp: subnetTemplate.GetIps()[idx],
					// NOTE: We don't need public IP for quotas reasons now
					PublicIp: false,
					//TODO: Add cloud init for worker
					CloudInit:  nil,
					ExternalIp: nil,
					Labels:     worker.GetMetadata(),
				}
			}
			unitOfWork.Commit()
			return &crossplane.Deployment_Template{
				Identifier: &crossplane.Identifier{
					Id:             ctx.StepRunId(),
					Name:           fmt.Sprintf("%s-%s", DefaultDeploymentName, ctx.StepRunId()),
					SupportedCloud: input.GetCommon().GetSupportedCloud(),
				},
				NetworkTemplate: networkTemplate,
				VmTemplates:     vms,
				Labels: map[string]string{
					RunIdLableName: ctx.StepRunId(),
				},
			}, nil
		}),
		hatchetLib.WithSkipIf(condition.Conditions()),
	)

	/*
		Build deployments
	*/
	buildDeploymentsTask := workflow.NewTask(
		BuildDeploymentsTaskName,
		hatchet_ext.PTask(logicalProvisionTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_Provision_Input,
			parentOutput *crossplane.Deployment_Template,
		) (*crossplane.Deployment, error) {
			err := parentOutput.Validate()
			if err != nil {
				return nil, fmt.Errorf("failed to validate %s output: %w", LogicalProvisionTaskName, err)
			}
			ctx.Log("Building Deployments")
			newDeployment, err := deps.Factory.CreateNewDeployment(parentOutput)
			if err != nil {
				return nil, fmt.Errorf("failed to create deployment: %w", err)
			}
			if err := deps.QuotaManager.ReserveQuotas(ctx, deployment.GetDeploymentUsingQuotas(newDeployment)); err != nil {
				return nil, fmt.Errorf("failed to reserve quotas: %w", err)
			}
			return newDeployment, nil
		}),
	)

	/*
		Reserve quotas in manager
	*/
	reserveQuotasTask := workflow.NewTask(
		ReserveQuotasTaskName,
		hatchet_ext.PTask(buildDeploymentsTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_Provision_Input,
			parentOutput *crossplane.Deployment,
		) (*crossplane.Deployment, error) {
			if err := deps.QuotaManager.ReserveQuotas(ctx, deployment.GetDeploymentUsingQuotas(parentOutput)); err != nil {
				return nil, fmt.Errorf("failed to reserve quotas: %w", err)
			}
			return parentOutput, nil
		}),
		hatchetLib.WithExecutionTimeout(1*time.Minute),
		hatchetLib.WithParents(buildDeploymentsTask),
	)

	/*
		Create deployments
	*/
	createDeploymentTask := workflow.NewTask(
		CreateDeploymentTaskName,
		hatchet_ext.PTask(reserveQuotasTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_Provision_Input,
			parentOutput *crossplane.Deployment,
		) (*crossplane.Deployment, error) {
			if _, err := deps.CrossplaneSvc.CreateDeployment(ctx, parentOutput); err != nil {
				return nil, fmt.Errorf("failed to create deployment: %w", err)
			}
			return parentOutput, nil
		}),
		hatchetLib.WithExecutionTimeout(2*time.Minute),
		hatchetLib.WithParents(reserveQuotasTask),
	)
	/*
		Wait for deployments to be ready
	*/
	waitDeploymentTask := workflow.NewTask(
		WaitDeploymentTaskName,
		hatchet_ext.PTask(createDeploymentTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_Provision_Input,
			parentOutput *crossplane.Deployment,
		) (*crossplane.Deployment, error) {
			for {
				select {
				case <-ctx.Done():
					return parentOutput, errors.Join(ctx.Err(), fmt.Errorf(
						"timeout waiting for deployment resources ready %s",
						parentOutput.GetTemplate().GetIdentifier().GetName(),
					))
				default:
					deploymentWithStatus, err := deps.CrossplaneSvc.ProcessDeploymentStatus(ctx, parentOutput)
					if err != nil {
						return parentOutput, err
					}
					if !crossplaneLib.IsResourcesReady(deployment.GetDeploymentResources(deploymentWithStatus)) {
						continue
					}
					return deploymentWithStatus, nil
				}
			}
		}),
		hatchetLib.WithExecutionTimeout(10*time.Minute),
		hatchetLib.WithParents(createDeploymentTask),
	)
	/*
		Wait for workers to be ready in Hatchet
	*/
	_ = workflow.NewTask(
		WaitWorkerInHatchet,
		hatchet_ext.PTask(waitDeploymentTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_Provision_Input,
			parentOutput *crossplane.Deployment,
		) (*hatchet.Workflows_Provision_Output, error) {
			workers := input.GetEdgeWorkersSet().GetEdgeWorkers()
			err = waitMultipleWorkersUp(ctx, c, workers...)
			if err != nil {
				return nil, err
			}
			var deployedWorkers []*hatchet.DeployedEdgeWorker
			for _, worker := range workers {
				deplVm, ok := lo.Find(parentOutput.GetVms(), func(vm *crossplane.Vm) bool {
					return vm.GetTemplate().GetIdentifier().GetName() == worker.GetWorkerName()
				})
				if !ok {
					return nil, fmt.Errorf("failed to find VM for worker %s", worker.GetWorkerName())
				}
				deployedWorkers = append(deployedWorkers, &hatchet.DeployedEdgeWorker{
					Worker:     worker,
					Deployment: deplVm,
				})
			}
			return &hatchet.Workflows_Provision_Output{
				Deployment: parentOutput,
				DeployedEdgeWorkers: &hatchet.DeployedEdgeWorkersSet{
					DeployedEdgeWorkers: deployedWorkers,
				},
			}, nil
		}),
		hatchetLib.WithExecutionTimeout(1*time.Minute),
		hatchetLib.WithParents(waitDeploymentTask),
	)
	/*
		Destroy deployments on failure
	*/
	workflow.OnFailure(func(
		ctx hatchetLib.Context,
		input hatchet.Workflows_Provision_Input,
	) (FailureHandlerOutput, error) {
		stepErrors := ctx.StepRunErrors()
		var errorDetails string
		for stepName, errorMsg := range stepErrors {
			ctx.Log(fmt.Sprintf("Multi-step: Step '%s' failed with error: %s", stepName, errorMsg))
			errorDetails += stepName + ": " + errorMsg + "; "
		}
		retErr := func(handled bool, err error) (FailureHandlerOutput, error) {
			return FailureHandlerOutput{
				FailureHandled: handled,
				ErrorDetails:   "Failed to handle deployments: " + err.Error(),
			}, nil
		}
		var readyDeployment *crossplane.Deployment
		if err := ctx.ParentOutput(buildDeploymentsTask, &readyDeployment); err == nil {
			return retErr(false, fmt.Errorf("failed to get %s output", BuildDeploymentsTaskName))
		}
		if err := deps.FallbackDestroyDeployment(ctx, readyDeployment); err != nil {
			return retErr(false, err)
		}
		if err != nil {
			return retErr(false, err)
		}
		return retErr(true, nil)
	})

	return workflow, nil
}
