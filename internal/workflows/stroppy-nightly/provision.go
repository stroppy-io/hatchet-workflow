package stroppy_nightly

import (
	"fmt"
	"time"

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
		"nightly-cloud-stroppy",
		hatchetLib.WithWorkflowDescription("Nightly Cloud Stroppy Workflow"),
	)
	buildDeploymentsTask := workflow.NewTask("build-deployments",
		func(ctx hatchetLib.Context, input *hatchet.NightlyCloudStroppyRequest) (*hatchet.ProvisionCloudResponse, error) {
			deployments, network, err := provision.BuildDeployments(
				ctx,
				ulid.Make().String(),
				quotaManager,
				networkManager,
				builder,
				input,
			)
			if err != nil {
				return nil, err
			}
			return &hatchet.ProvisionCloudResponse{
				RunId:       ulid.Make().String(),
				Deployments: deployments,
				Network:     network,
			}, nil
		},
	)
	createDeploymentTask := workflow.NewTask("create-deployments",
		func(ctx hatchetLib.Context, input *hatchet.ProvisionCloudResponse) (*hatchet.ProvisionCloudResponse, error) {
			err := input.Validate()
			if err != nil {
				return nil, err
			}
			err = provision.CreateDeployment(ctx, crossplaneSvc, input.Deployments)
			if err != nil {
				return nil, err
			}
			return input, nil
		},
		hatchetLib.WithExecutionTimeout(2*time.Minute),
		hatchetLib.WithParents(buildDeploymentsTask),
	)
	waitDeploymentTask := workflow.NewTask("wait-deployments",
		func(ctx hatchetLib.Context, input *hatchet.ProvisionCloudResponse) (*hatchet.ProvisionCloudResponse, error) {
			err := provision.WaitDeployment(ctx, crossplaneSvc, input.Deployments)
			if err != nil {
				return nil, err
			}
			return input, nil
		},
		hatchetLib.WithExecutionTimeout(10*time.Minute),
		hatchetLib.WithParents(createDeploymentTask),
	)
	runStroppyTask := workflow.NewTask(
		"run-stroppy-test",
		func(ctx hatchetLib.Context, input *hatchet.ProvisionCloudResponse) (*hatchet.NightlyCloudStroppyResponse, error) {
			ctx.Log("Running Stroppy Test")
			var workflowInput hatchet.NightlyCloudStroppyRequest
			err = ctx.WorkflowInput(&workflowInput)
			if err != nil {
				return nil, fmt.Errorf("failed to get workflow input: %w", err)
			}
			postgresResult, err := NightlyCloudStroppyRunPostgresTask(input.GetRunId(), c).Run(ctx, &hatchet.InstallPostgresParams{
				RunId:    input.GetRunId(),
				Version:  workflowInput.GetPostgresVersion(),
				Settings: workflowInput.GetPostgresSettings(),
				//EnableOrioledb: false,
				//OrioledbSettings: map[string]string{},
			})
			if err != nil {
				return nil, err
			}
			var postgresOutput hatchet.InstallPostgresParams
			err = postgresResult.Into(&postgresOutput)
			if err != nil {
				return nil, fmt.Errorf("failed to get child workflow result: %w", err)
			}
			runStroppyResult, err := NightlyCloudStroppyRunWorkflow(input.GetRunId(), c).
				Run(ctx, &hatchet.RunStroppyParams{
					RunId: input.GetRunId(),
					//BinaryPath: "" // NOTE: Not set cause installer chose it by himself
					Version:      workflowInput.GetStroppyVersion(),
					WorkloadName: workflowInput.GetStroppyWorkflowName(),
					// WARN: This is the Postgres URL for the first IP in the network by provisioning design
					ConnectionString: install.DefaultConfig().PostgresUrlByIp(input.GetNetwork().GetIps()[0].GetValue()),
					Env:              workflowInput.GetStroppyEnv(),
				})
			if err != nil {
				return nil, fmt.Errorf("failed to run Stroppy workflow: %w", err)
			}
			var runStroppyOutput hatchet.RunStroppyResponse
			err = runStroppyResult.TaskOutput(stroppyRunTaskName(input.GetRunId())).Into(&runStroppyOutput)
			if err != nil {
				return nil, fmt.Errorf("failed to get child workflow result: %w", err)
			}
			return &hatchet.NightlyCloudStroppyResponse{
				RunId:       input.GetRunId(),
				Deployments: input.GetDeployments(),
				GrafanaUrl:  runStroppyOutput.GetGrafanaUrl(),
			}, nil
		},
		hatchetLib.WithParents(waitDeploymentTask),
	)
	workflow.NewTask(
		"destroy-deployments",
		func(ctx hatchetLib.Context, input *hatchet.NightlyCloudStroppyResponse) (*hatchet.NightlyCloudStroppyResponse, error) {
			err := provision.DestroyDeployments(
				ctx,
				crossplaneSvc,
				quotaManager,
				networkManager,
				input.Deployments,
				input.GetUsedNetwork(),
			)
			if err != nil {
				return nil, err
			}
			return input, nil
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
		var step1Output *hatchet.ProvisionCloudResponse
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
