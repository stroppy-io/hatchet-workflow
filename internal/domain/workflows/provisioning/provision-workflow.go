package provisioning

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cenkalti/backoff/v4"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/oklog/ulid/v2"
	"github.com/sourcegraph/conc/pool"
	crossplaneLib "github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane/k8s"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/provider/yandex"
	"github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/core/uow"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/managers"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

const (
	DefaultCrossplaneNamespace = "crossplane-system"
	DefaultNetworkName         = "stroppy-crossplane-net"
	DefaultNetworkId           = "enp7b429s2br5pja0jci"
	DefaultVmZone              = "ru-central1-d"
	DefaultVmPlatformId        = "standard-v2"
)

const (
	WorkflowName             = "provision-cloud"
	BuildDeploymentsTaskName = "build-deployments"
	CreateDeploymentTaskName = "create-deployments"
	WaitDeploymentTaskName   = "wait-deployments"
	WaitWorkerInHatchet      = "wait-worker-in-hatchet-ext"
)

type FailureInput struct {
	Message     string `json:"message"`
	ShouldFail  bool   `json:"should_fail"`
	FailureType string `json:"failure_type"`
}
type FailureHandlerOutput struct {
	FailureHandled bool   `json:"failure_handled"`
	ErrorDetails   string `json:"error_details"`
	OriginalInput  string `json:"original_input"`
}

func ProvisionWorkflow(
	c *hatchetLib.Client,
) (*hatchetLib.Workflow, error) {
	k8sConfigPath := os.Getenv(K8SConfigPath)
	if k8sConfigPath == "" {
		return nil, fmt.Errorf("environment variable %s is not set", K8SConfigPath)
	}
	builder := deployment.NewBuilder(map[crossplane.SupportedCloud]deployment.DetailsBuilder{
		crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX: yandex.NewCloudBuilder(&yandex.ProviderConfig{
			K8sNamespace:        DefaultCrossplaneNamespace,
			DefaultNetworkName:  DefaultNetworkName,
			DefaultNetworkId:    DefaultNetworkId,
			DefaultVmZone:       DefaultVmZone,
			DefaultVmPlatformId: DefaultVmPlatformId,
		}),
	})
	valkeyClient, err := valkeyFromEnv()
	if err != nil {
		return nil, err
	}
	networkManager, err := managers.NewNetworkManager(valkeyClient, managers.DefaultNetworkConfig())
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
	workflow := c.NewWorkflow(
		WorkflowName,
		hatchetLib.WithWorkflowDescription("Provision Cloud Workflow"),
	)
	/*
		Build deployments
	*/
	buildDeploymentsTask := workflow.NewTask(
		BuildDeploymentsTaskName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_Provision_Input,
		) (*crossplane.DeploymentSet, error) {
			ctx.Log("Building Deployments")
			err := input.Validate()
			if err != nil {
				return nil, fmt.Errorf("failed to validate input: %w", err)
			}
			unitOfWork := uow.UnitOfWork()
			var network *crossplane.Network
			if input.GetRequest().GetUseExistingNetwork() != nil {
				network = input.GetRequest().GetUseExistingNetwork()
			} else {
				cidrWithIps, err := networkManager.ReserveNetwork(ctx, len(input.GetRequest().GetRequests()))
				if err != nil {
					return nil, fmt.Errorf("failed to reserve network: %w", err)
				}
				id := ulid.Make().String()
				network = &crossplane.Network{
					Id:          id,
					Name:        fmt.Sprintf("stroppy-generated-network-%s", id),
					CidrWithIps: cidrWithIps,
				}
				unitOfWork.Add("network", func() error {
					return networkManager.FreeNetwork(ctx, network.GetCidrWithIps())
				})
			}
			deploymentsSet, err := deployment.NewDeploymentSetFromRequest(
				network,
				input.GetRequest(),
				builder,
			)
			if err != nil {
				return nil, unitOfWork.Rollback(fmt.Errorf("failed to create deployment set: %w", err))
			}
			quotas := make([]*crossplane.Quota, 0)
			for _, depl := range deploymentsSet.GetDeployments() {
				quotas = append(quotas, depl.GetCloudDetails().GetUsingQuotas()...)
			}
			if err := quotaManager.ReserveQuotas(ctx, quotas); err != nil {
				return nil, unitOfWork.Rollback(fmt.Errorf("failed to reserve quotas: %w", err))
			}
			unitOfWork.Commit()
			return deploymentsSet, nil
		}),
	)
	/*
		Create deployments
	*/
	createDeploymentTask := workflow.NewTask(
		CreateDeploymentTaskName,
		hatchet_ext.Ptask(buildDeploymentsTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_Provision_Input,
			parentOutput *crossplane.DeploymentSet,
		) (*crossplane.DeploymentSet, error) {
			for _, depl := range parentOutput.GetDeployments() {
				if _, err := crossplaneSvc.CreateDeployment(ctx, depl); err != nil {
					return nil, fmt.Errorf("failed to create deployment: %w", err)
				}
			}
			return parentOutput, nil
		}),
		hatchetLib.WithExecutionTimeout(2*time.Minute),
		hatchetLib.WithParents(buildDeploymentsTask),
	)
	/*
		Wait for deployments to be ready
	*/
	waitDeploymentTask := workflow.NewTask(
		WaitDeploymentTaskName,
		hatchet_ext.Ptask(createDeploymentTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_Provision_Input,
			parentOutput *crossplane.DeploymentSet,
		) (*crossplane.DeploymentSet, error) {
			waitPool := pool.New().WithContext(ctx).WithFailFast().WithCancelOnError()
			for _, depl := range parentOutput.GetDeployments() {
				waitPool.Go(func(ctx context.Context) error {
					for {
						select {
						case <-ctx.Done():
							return errors.Join(ctx.Err(), fmt.Errorf(
								"timeout waiting for deployment resources ready %s",
								depl.GetId(),
							))
						default:
							deploymentWithStatus, err := crossplaneSvc.ProcessDeploymentStatus(ctx, depl)
							if err != nil {
								continue
							}
							if !crossplaneLib.IsResourcesReady(deploymentWithStatus.GetCloudDetails().GetResources()) {
								continue
							}
							return nil
						}
					}
				})
			}
			return parentOutput, waitPool.Wait()
		}),
		hatchetLib.WithExecutionTimeout(10*time.Minute),
		hatchetLib.WithParents(createDeploymentTask),
	)
	/*
		Wait for workers to be ready in Hatchet
	*/
	_ = workflow.NewTask(
		WaitWorkerInHatchet,
		hatchet_ext.Ptask(waitDeploymentTask, func(
			ctx hatchetLib.Context,
			input *hatchet.Tasks_Provision_Input,
			parentOutput *crossplane.DeploymentSet,
		) (*hatchet.Tasks_Provision_Output, error) {
			err = waitMultipleWorkersUp(ctx, c, input.GetHatchetWorkersNames()...)
			if err != nil {
				return nil, err
			}
			return &hatchet.Tasks_Provision_Output{
				DeploymentSet: parentOutput,
			}, nil
		}),
		hatchetLib.WithExecutionTimeout(1*time.Minute),
		hatchetLib.WithParents(waitDeploymentTask),
	)
	/*
		Destroy deployments on failure
	*/
	workflow.OnFailure(func(ctx hatchetLib.Context, input FailureInput) (FailureHandlerOutput, error) {
		ctx.Log(fmt.Sprintf("Provision Cloud Workflow failed: %s", input.Message))
		stepErrors := ctx.StepRunErrors()
		var errorDetails string
		for stepName, errorMsg := range stepErrors {
			ctx.Log(fmt.Sprintf("Multi-step: Step '%s' failed with error: %s", stepName, errorMsg))
			errorDetails += stepName + ": " + errorMsg + "; "
		}
		var step1Output *crossplane.DeploymentSet
		if err := ctx.StepOutput(BuildDeploymentsTaskName, &step1Output); err == nil {
			ctx.Log(fmt.Sprintf("First step completed successfully with: %s", step1Output))
		}
		err := backoff.Retry(func() error {
			for _, depl := range step1Output.GetDeployments() {
				if err := crossplaneSvc.DestroyDeployment(ctx, depl); err != nil {
					return err
				}
				if err := quotaManager.FreeQuotas(ctx, depl.GetCloudDetails().GetUsingQuotas()); err != nil {
					return err
				}
			}
			if err := networkManager.FreeNetwork(ctx, step1Output.GetNetwork().GetCidrWithIps()); err != nil {
				return err
			}
			return nil
		}, backoff.WithContext(
			backoff.WithMaxRetries(backoff.NewConstantBackOff(10*time.Second), 3),
			ctx,
		))
		if err != nil {
			return FailureHandlerOutput{
				FailureHandled: false,
				ErrorDetails:   "Failed to destroy deployments: " + err.Error(),
				OriginalInput:  input.Message,
			}, nil
		}
		return FailureHandlerOutput{
			FailureHandled: true,
			ErrorDetails:   "Multi-step workflow failed: " + errorDetails,
			OriginalInput:  input.Message,
		}, nil
	})

	return workflow, nil
}
