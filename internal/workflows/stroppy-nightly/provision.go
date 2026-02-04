package stroppy_nightly

import (
	"fmt"
	"time"

	"github.com/hatchet-dev/hatchet/pkg/client/create"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog/log"
	crossplaneLib "github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane/k8s"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/managers"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/provider/yandex"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/install"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/provision"
	valkeygo "github.com/valkey-io/valkey-go"
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

const (
	DefaultCrossplaneNamespace = "crossplane-system"
	DefaultNetworkName         = "stroppy-crossplane-net"
	DefaultNetworkId           = "enp7b429s2br5pja0jci"
	DefaultVmZone              = "ru-central1-d"
	DefaultVmPlatformId        = "standard-v2"
)

func RuntimeStroppyWorkerName(runId string) string {
	return "stroppy-worker-" + runId
}

func RuntimePostgresWorkerName(runId string) string {
	return "postgres-worker-" + runId
}

const (
	WorkflowName               = "nightly-cloud-stroppy"
	BuildDeploymentsTaskName   = "build-deployments"
	CreateDeploymentTaskName   = "create-deployments"
	WaitDeploymentTaskName     = "wait-deployments"
	WaitWorkerInHatchet        = "wait-worker-in-hatchet"
	RunStroppyTaskName         = "run-stroppy-test"
	DestroyDeploymentsTaskName = "destroy-deployments"
)

func getParentProvisionCloudParams(ctx hatchetLib.Context, parentTask create.NamedTask) (hatchet.ProvisionCloudParams, error) {
	var provisionCloudParams hatchet.ProvisionCloudParams
	if err := ctx.ParentOutput(parentTask, &provisionCloudParams); err != nil {
		return hatchet.ProvisionCloudParams{}, err
	}
	err := provisionCloudParams.Validate()
	if err != nil {
		return hatchet.ProvisionCloudParams{}, err
	}
	return provisionCloudParams, nil
}

func NightlyCloudStroppyProvisionWorkflow(
	c *hatchetLib.Client,
	valkeyClient valkeygo.Client,
	k8sConfigPath string,
) (*hatchetLib.Workflow, error) {
	builder := deployment.NewBuilder(map[crossplane.SupportedCloud]deployment.YamlBuilder{
		crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX: yandex.NewCloudBuilder(&yandex.ProviderConfig{
			K8sNamespace:        DefaultCrossplaneNamespace,
			DefaultNetworkName:  DefaultNetworkName,
			DefaultNetworkId:    DefaultNetworkId,
			DefaultVmZone:       DefaultVmZone,
			DefaultVmPlatformId: DefaultVmPlatformId,
		}),
	})
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
		hatchetLib.WithWorkflowDescription("Nightly Cloud Stroppy Workflow"),
	)
	buildDeploymentsTask := workflow.NewTask(BuildDeploymentsTaskName,
		func(ctx hatchetLib.Context, input hatchet.NightlyCloudStroppy) (hatchet.ProvisionCloudParams, error) {
			ctx.Log("Building Deployments")
			err := input.Validate()
			if err != nil {
				return hatchet.ProvisionCloudParams{}, fmt.Errorf("failed to validate input: %w", err)
			}
			deployments, network, err := provision.BuildDeployments(
				ctx,
				ulid.Make().String(),
				quotaManager,
				networkManager,
				builder,
				&input,
			)
			if err != nil {
				return hatchet.ProvisionCloudParams{}, err
			}
			return hatchet.ProvisionCloudParams{
				RunId:       ulid.Make().String(),
				Deployments: deployments,
				Network:     network,
			}, nil
		},
	)
	createDeploymentTask := workflow.NewTask(CreateDeploymentTaskName,
		func(ctx hatchetLib.Context, input hatchet.NightlyCloudStroppy) (hatchet.ProvisionCloudParams, error) {
			ctx.Log("Creating Deployments")
			buildOutput, err := getParentProvisionCloudParams(ctx, buildDeploymentsTask)
			if err != nil {
				return hatchet.ProvisionCloudParams{}, err
			}
			result, err := provision.CreateDeployment(ctx, crossplaneSvc, buildOutput.Deployments)
			if err != nil {
				return hatchet.ProvisionCloudParams{}, err
			}
			return hatchet.ProvisionCloudParams{
				RunId:       buildOutput.GetRunId(),
				Deployments: result,
				Network:     buildOutput.GetNetwork(),
			}, nil
		},
		hatchetLib.WithExecutionTimeout(2*time.Minute),
		hatchetLib.WithParents(buildDeploymentsTask),
	)
	waitDeploymentTask := workflow.NewTask(WaitDeploymentTaskName,
		func(ctx hatchetLib.Context, input hatchet.NightlyCloudStroppy) (hatchet.ProvisionCloudParams, error) {
			ctx.Log("Waiting for Deployments")
			createOutput, err := getParentProvisionCloudParams(ctx, createDeploymentTask)
			if err != nil {
				return hatchet.ProvisionCloudParams{}, err
			}
			result, err := provision.WaitDeployment(ctx, crossplaneSvc, createOutput.Deployments)
			if err != nil {
				return hatchet.ProvisionCloudParams{}, err
			}
			return hatchet.ProvisionCloudParams{
				RunId:       createOutput.GetRunId(),
				Deployments: result,
				Network:     createOutput.GetNetwork(),
			}, nil
		},
		hatchetLib.WithExecutionTimeout(10*time.Minute),
		hatchetLib.WithParents(createDeploymentTask),
	)
	waitWorkerInHatchet := workflow.NewTask(WaitWorkerInHatchet,
		func(ctx hatchetLib.Context, input hatchet.NightlyCloudStroppy) (hatchet.NightlyCloudStroppyResult, error) {
			ctx.Log("Running Stroppy Test")
			var waitOutput hatchet.ProvisionCloudParams
			if err := ctx.ParentOutput(waitDeploymentTask, &waitOutput); err != nil {
				return hatchet.NightlyCloudStroppyResult{}, err
			}
			err := waitOutput.Validate()
			if err != nil {
				return hatchet.NightlyCloudStroppyResult{}, err
			}
			err = waitMultipleWorkersUp(ctx, c,
				RuntimeStroppyWorkerName(waitOutput.GetRunId()),
				RuntimePostgresWorkerName(waitOutput.GetRunId()),
			)
			if err != nil {
				return hatchet.NightlyCloudStroppyResult{}, err
			}
			return hatchet.NightlyCloudStroppyResult{}, nil
		},
		hatchetLib.WithExecutionTimeout(1*time.Minute),
		hatchetLib.WithParents(waitDeploymentTask),
	)
	runStroppyTask := workflow.NewTask(RunStroppyTaskName,
		func(ctx hatchetLib.Context, input hatchet.NightlyCloudStroppy) (hatchet.NightlyCloudStroppyResult, error) {
			ctx.Log("Running Stroppy Test")
			var waitOutput hatchet.ProvisionCloudParams
			if err := ctx.ParentOutput(waitWorkerInHatchet, &waitOutput); err != nil {
				return hatchet.NightlyCloudStroppyResult{}, err
			}
			err := waitOutput.Validate()
			if err != nil {
				return hatchet.NightlyCloudStroppyResult{}, err
			}
			postgresResult, err := NightlyCloudStroppyRunPostgresTask(waitOutput.GetRunId(), c).
				Run(ctx, &hatchet.InstallPostgresParams{
					RunId:    waitOutput.GetRunId(),
					Version:  input.GetPostgresVersion(),
					Settings: input.GetPostgresSettings(),
					//EnableOrioledb: false,
					//OrioledbSettings: map[string]string{},
				})
			if err != nil {
				return hatchet.NightlyCloudStroppyResult{}, err
			}
			var postgresOutput hatchet.InstallPostgresParams
			err = postgresResult.Into(&postgresOutput)
			if err != nil {
				return hatchet.NightlyCloudStroppyResult{}, fmt.Errorf("failed to get child workflow result: %w", err)
			}
			runStroppyResult, err := NightlyCloudStroppyRunWorkflow(waitOutput.GetRunId(), c).
				Run(ctx, &hatchet.RunStroppyParams{
					RunId: waitOutput.GetRunId(),
					//BinaryPath: "" // NOTE: Not set cause installer chose it by himself
					Version:      input.GetStroppyVersion(),
					WorkloadName: input.GetStroppyWorkloadName(),
					// WARN: This is the Postgres URL for the first IP in the network by provisioning design
					ConnectionString: install.DefaultConfig().PostgresUrlByIp(waitOutput.GetNetwork().GetIps()[0].GetValue()),
					Env:              input.GetStroppyEnv(),
				})
			if err != nil {
				return hatchet.NightlyCloudStroppyResult{}, fmt.Errorf("failed to run Stroppy workflow: %w", err)
			}
			var runStroppyOutput hatchet.RunStroppyResponse
			err = runStroppyResult.TaskOutput(stroppyRunTaskName(waitOutput.GetRunId())).Into(&runStroppyOutput)
			if err != nil {
				return hatchet.NightlyCloudStroppyResult{}, fmt.Errorf("failed to get child workflow result: %w", err)
			}
			return hatchet.NightlyCloudStroppyResult{
				RunId:       waitOutput.GetRunId(),
				Deployments: waitOutput.GetDeployments(),
				GrafanaUrl:  runStroppyOutput.GetGrafanaUrl(),
			}, nil
		},
		hatchetLib.WithParents(waitWorkerInHatchet),
	)
	workflow.NewTask(DestroyDeploymentsTaskName,
		func(ctx hatchetLib.Context, input hatchet.NightlyCloudStroppy) (hatchet.NightlyCloudStroppyResult, error) {
			var runStroppyOutput hatchet.NightlyCloudStroppyResult
			if err := ctx.ParentOutput(runStroppyTask, &runStroppyOutput); err != nil {
				return hatchet.NightlyCloudStroppyResult{}, err
			}
			err := provision.DestroyDeployments(
				ctx,
				crossplaneSvc,
				quotaManager,
				networkManager,
				runStroppyOutput.Deployments,
				runStroppyOutput.GetUsedNetwork(),
			)
			if err != nil {
				return hatchet.NightlyCloudStroppyResult{}, err
			}
			return runStroppyOutput, nil
		},
		hatchetLib.WithExecutionTimeout(2*time.Minute),
		hatchetLib.WithParents(runStroppyTask),
	)

	workflow.OnFailure(func(ctx hatchetLib.Context, input FailureInput) (FailureHandlerOutput, error) {
		log.Printf("Multi-step failure handler called for input: %s", input.Message)

		stepErrors := ctx.StepRunErrors()
		var errorDetails string
		for stepName, errorMsg := range stepErrors {
			log.Printf("Multi-step: Step '%s' failed with error: %s", stepName, errorMsg)
			errorDetails += stepName + ": " + errorMsg + "; "
		}
		// Access successful step outputs for cleanup
		var step1Output *hatchet.ProvisionCloudParams
		if err := ctx.StepOutput("build-deployments", &step1Output); err == nil {
			log.Printf("First step completed successfully with: %s", step1Output.RunId)
		}
		errr := provision.DestroyDeployments(
			ctx,
			crossplaneSvc,
			quotaManager,
			networkManager,
			step1Output.Deployments,
			step1Output.GetNetwork(),
		)
		if errr != nil {
			return FailureHandlerOutput{
				FailureHandled: false,
				ErrorDetails:   "Failed to destroy deployments: " + errr.Error(),
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
