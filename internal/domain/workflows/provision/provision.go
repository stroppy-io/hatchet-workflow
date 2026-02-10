package provision

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/hatchet-dev/hatchet/pkg/worker/condition"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/samber/lo"
	crossplaneLib "github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	"github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/core/uow"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/scripting"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"go.uber.org/zap/zapcore"
)

const (
	DefaultCrossplaneNamespace consts.DefaultValue = "crossplane-system"
	DefaultNetworkName         consts.DefaultValue = "stroppy-crossplane-net"
	DefaultSubnetName          consts.DefaultValue = "stroppy-crossplane-subnet"
	//DefaultVmName            consts.DefaultValue   = "stroppy-crossplane-vm"
	DefaultDeploymentName     consts.DefaultValue = "stroppy-crossplane-deployment"
	DefaultNetworkId          consts.DefaultValue = "enp7b429s2br5pja0jci"
	DefaultUbuntuImageId      consts.DefaultValue = "fd82pkek8uu0ejjkh4vn"
	DefaultVmZone             consts.DefaultValue = "ru-central1-d"
	DefaultVmPlatformId       consts.DefaultValue = "standard-v2"
	DefaultSubnetBaseCidr     consts.DefaultValue = "10.2.0.0/16"
	DefaultSubnetBasePrefix                       = 24
	DefaultEdgeWorkerUserName consts.DefaultValue = "stroppy-edge-worker"
	DefaultEdgeWorkerSshKey   consts.DefaultValue = "stroppy-edge-worker-ssh-key"
)

const (
	CloudProvisionWorkflowName consts.Str = "cloud-provision"

	LogicalProvisionTaskName consts.Str = "logical-provision"
	BuildDeploymentsTaskName consts.Str = "build-deployments"
	ReserveQuotasTaskName    consts.Str = "reserve-quotas"
	CreateDeploymentTaskName consts.Str = "create-deployments"
	WaitDeploymentTaskName   consts.Str = "wait-deployments"
	WaitWorkerInHatchet      consts.Str = "wait-worker-in-hatchet"
)

const (
	RunIdLableName consts.Str = "run_id"
)

const (
	DefaultUserGroupName consts.DefaultValue = "stroppy-edge-worker"
	DefaultUserSudo      bool                = true
	DefaultUserSudoRules consts.DefaultValue = "ALL=(ALL) NOPASSWD:ALL"
	DefautltUserShell    consts.DefaultValue = "/bin/bash"
)

const (
	HatchetServerUrlKey         consts.EnvKey = "HATCHET_CLIENT_SERVER_URL"
	HatchetServerHostPortKey    consts.EnvKey = "HATCHET_CLIENT_HOST_PORT"
	HatchetClientTokenKey       consts.EnvKey = "HATCHET_CLIENT_TOKEN"
	HatchetClientTlsStrategyKey consts.EnvKey = "HATCHET_CLIENT_TLS_STRATEGY"

	HatchetEdgeWorkerUserName consts.EnvKey = "HATCHET_EDGE_WORKER_USER_NAME"
	HatchetEdgeWorkerSshKey   consts.EnvKey = "HATCHET_EDGE_WORKER_SSH_KEY"
	HatchetEdgeWorkerPublicIp consts.EnvKey = "HATCHET_EDGE_WORKER_PUBLIC_IP"

	HatchetClientTlsStrategyNone consts.Str = "none"
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
			if input.GetCommon().GetSupportedCloud() == crossplane.SupportedCloud_SUPPORTED_CLOUD_UNSPECIFIED {
				return nil, fmt.Errorf("unsupported cloud: %s", input.GetCommon().GetSupportedCloud())
			}
			workers := input.GetEdgeWorkersSet().GetEdgeWorkers()
			unitOfWork := uow.UnitOfWork()
			networkTemplate := &crossplane.Network_Template{
				Identifier: &crossplane.Identifier{
					// NOTE: Now we use one predefined network for all subnets
					Id:             ids.NewUlid().Lower().String(),
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
			ctx.Log(fmt.Sprintf("Reserved network %s (cidr:%s)", networkTemplate.GetIdentifier().GetName(), subnetTemplate.GetCidr().GetValue()))
			// NOTE: Add rollback for network if next step fails
			unitOfWork.Add("rollback network", func() error {
				return deps.NetworkManager.FreeNetwork(ctx, networkTemplate.GetIdentifier(), subnetTemplate)
			})
			networkTemplate.Subnets = append(networkTemplate.Subnets, subnetTemplate)

			var hatchetServerRefKey string
			var hatchetServerRef string
			switch input.GetCommon().GetHatchetServer().GetServer().(type) {
			case *hatchet.HatchetServer_HostPort_:
				hatchetServerRefKey = HatchetServerHostPortKey
				hatchetServerRef = net.JoinHostPort(
					input.GetCommon().GetHatchetServer().GetHostPort().GetHost(),
					input.GetCommon().GetHatchetServer().GetHostPort().GetPort(),
				)
			case *hatchet.HatchetServer_Url:
				hatchetServerRefKey = HatchetServerUrlKey
				hatchetServerRef = input.GetCommon().GetHatchetServer().GetUrl()
			default:
				return nil, fmt.Errorf("unsupported hatchet server type")
			}
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
					// NOTE: For debugging proposes we can set public IP for VMs by setting HATCHET_EDGE_WORKER_PUBLIC_IP=true
					PublicIp: os.Getenv(HatchetEdgeWorkerPublicIp) == "true",
					// TODO: Add cloud init for worker
					CloudInit: scripting.InstallEdgeWorkerCloudInit(
						scripting.WithUsers([]*crossplane.User{
							{
								Name: defaults.StringOrDefault(
									os.Getenv(HatchetEdgeWorkerUserName),
									DefaultEdgeWorkerUserName,
								),
								Shell: DefautltUserShell,
								Groups: []string{
									DefaultUserGroupName,
								},
								Sudo:      DefaultUserSudo,
								SudoRules: DefaultUserSudoRules,
								SshAuthorizedKeys: []string{
									defaults.StringOrDefault(
										os.Getenv(HatchetEdgeWorkerSshKey),
										DefaultEdgeWorkerSshKey,
									),
								},
							},
						}),
						// NOTE: We must chose between HatchetServerUrlKey and HatchetServerHostPortKey
						scripting.WithEnv(map[string]string{
							hatchetServerRefKey:              hatchetServerRef,
							edge.WorkerNameEnvKey:            worker.GetWorkerName(),
							edge.WorkerAcceptableTasksEnvKey: edge.TaskIdListToString(worker.GetAcceptableTasks()),
							HatchetClientTokenKey:            input.GetCommon().GetHatchetServer().GetToken(),
							// TODO: Add tls after domain access
							HatchetClientTlsStrategyKey: HatchetClientTlsStrategyNone,
							HatchetEdgeWorkerUserName: defaults.StringOrDefault(
								os.Getenv(HatchetEdgeWorkerUserName),
								DefaultEdgeWorkerUserName,
							),
							HatchetEdgeWorkerSshKey: defaults.StringOrDefault(
								os.Getenv(HatchetEdgeWorkerSshKey),
								DefaultEdgeWorkerSshKey,
							),
						}),
						scripting.WithAddEnv(map[string]string{
							logger.LevelEnvKey: defaults.StringOrDefault(
								os.Getenv(logger.LevelEnvKey),
								zapcore.InfoLevel.String(),
							),
							logger.LogModEnvKey: defaults.StringOrDefault(
								os.Getenv(logger.LogModEnvKey),
								logger.ProductionMod.String(),
							),
							logger.LogMappingEnvKey: os.Getenv(logger.LogMappingEnvKey),
							logger.LogSkipCallerEnvKey: defaults.StringOrDefault(
								os.Getenv(logger.LogSkipCallerEnvKey),
								"true",
							),
						}),
					),
					ExternalIp: nil,
					Labels:     worker.GetMetadata(),
				}
			}
			unitOfWork.Commit()
			deplId := ids.NewUlid().Lower().String()
			return &crossplane.Deployment_Template{
				Identifier: &crossplane.Identifier{
					Id:             deplId,
					Name:           fmt.Sprintf("%s-%s", DefaultDeploymentName, deplId),
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
			newDeployment, err := deps.Factory.CreateNewDeployment(parentOutput)
			if err != nil {
				return nil, fmt.Errorf("failed to create deployment: %w", err)
			}
			return newDeployment, nil
		}),
		hatchetLib.WithExecutionTimeout(1*time.Minute),
		hatchetLib.WithParents(logicalProvisionTask),
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
			svc, err := deps.GetDeploymentSvc(input.GetCommon().GetSupportedCloud())
			if err != nil {
				return nil, err
			}
			if _, err := svc.CreateDeployment(ctx, parentOutput); err != nil {
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
			svc, err := deps.GetDeploymentSvc(input.GetCommon().GetSupportedCloud())
			if err != nil {
				return parentOutput, err
			}
			for {
				select {
				case <-ctx.Done():
					return parentOutput, errors.Join(ctx.Err(), fmt.Errorf(
						"timeout waiting for deployment resources ready %s",
						parentOutput.GetTemplate().GetIdentifier().GetName(),
					))
				default:
					deploymentWithStatus, err := svc.ProcessDeploymentStatus(ctx, parentOutput)
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
				DeployedEdgeWorkers: &hatchet.DeployedEdgeWorkersSet{
					Deployment:          parentOutput,
					DeployedEdgeWorkers: deployedWorkers,
				},
			}, nil
		}),
		hatchetLib.WithExecutionTimeout(10*time.Minute),
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
		if err := ctx.ParentOutput(buildDeploymentsTask, &readyDeployment); err != nil {
			return retErr(false, fmt.Errorf("failed to get %s output", BuildDeploymentsTaskName))
		}
		if err := deps.FallbackDestroyDeployment(ctx, input.GetCommon().GetSupportedCloud(), readyDeployment); err != nil {
			return retErr(false, err)
		}
		if err != nil {
			return retErr(false, err)
		}
		return retErr(true, nil)
	})

	return workflow, nil
}
