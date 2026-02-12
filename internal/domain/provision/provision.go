package provision

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ips"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/deployment/scripting"
	edgeDomain "github.com/stroppy-io/hatchet-workflow/internal/domain/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
	"go.uber.org/zap/zapcore"
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

	HatchetClientTlsStrategyNone consts.Str = "none"

	metadataRoleKey          consts.ConstValue   = "METADATA_ROLE"
	metadataRunIdKey         consts.ConstValue   = "METADATA_RUN_ID"
	metadataRoleStroppyValue consts.DefaultValue = "stroppy"
	metadataDatabaseValue    consts.DefaultValue = "database"
	globalDeploymentName     consts.DefaultValue = "stroppy-test-run-deployment"
)

func stroppyMetadata(runId string) map[string]string {
	return map[string]string{
		metadataRoleKey:  metadataRoleStroppyValue,
		metadataRunIdKey: runId,
	}
}
func databaseMetadata(runId string) map[string]string {
	return map[string]string{
		metadataRoleKey:  metadataDatabaseValue,
		metadataRunIdKey: runId,
	}
}

type NetworkManager interface {
	ReserveNetwork(
		ctx context.Context,
		networkIdentifier *deployment.Identifier,
		baseCidr string,
		basePrefix int,
		ipCount int,
	) (*deployment.Network, error)
	FreeNetwork(
		ctx context.Context,
		network *deployment.Network,
	) error
}

type DeploymentService interface {
	CreateDeployment(
		ctx context.Context,
		depl *deployment.Deployment_Template,
	) (*deployment.Deployment, error)
	DestroyDeployment(
		ctx context.Context,
		depl *deployment.Deployment,
	) error
}

type ProvisionerService struct {
	networkManager    NetworkManager
	deploymentService DeploymentService
}

func NewProvisionerService(
	networkManager NetworkManager,
	deploymentService DeploymentService,
) *ProvisionerService {
	return &ProvisionerService{
		networkManager:    networkManager,
		deploymentService: deploymentService,
	}
}

func getDeploymentTarget(target *settings.SelectedTarget) deployment.Target {
	switch target.GetTarget().(type) {
	case *settings.SelectedTarget_DockerSettings:
		return deployment.Target_TARGET_DOCKER
	case *settings.SelectedTarget_YandexCloudSettings:
		return deployment.Target_TARGET_YANDEX_CLOUD
	default:
		return deployment.Target_TARGET_UNSPECIFIED
	}
}

func (p ProvisionerService) AcquireNetwork(
	ctx context.Context,
	testRunCtx *stroppy.TestRunContext,
) (*deployment.Network, error) {
	var count int
	switch testRunCtx.GetTest().GetDatabaseRef().(type) {
	case *stroppy.Test_DatabaseTemplate:
		count = RequiredIPCount(testRunCtx.GetTest().GetDatabaseTemplate())
	case *stroppy.Test_ConnectionString:
		count = 1
	}
	count += 1 // for stroppy deployment
	return p.networkManager.ReserveNetwork(
		ctx,
		&deployment.Identifier{
			Id:     ids.NewUlid().Lower().String(),
			Name:   fmt.Sprintf("stroppy-network-%s", testRunCtx.GetRunId()),
			Target: getDeploymentTarget(testRunCtx.GetSelectedTarget()),
		},
		"10.2.0.0/16", // now hardcoded for yandex cloud need get from settings
		24,            // now hardcoded for yandex cloud need get from settings
		count,
	)
}

func (p ProvisionerService) FreeNetwork(
	ctx context.Context,
	network *deployment.Network,
) error {
	return p.networkManager.FreeNetwork(ctx, network)
}

func (p ProvisionerService) PlanPlacementIntent(
	_ context.Context,
	template *database.Database_Template,
	network *deployment.Network,
) (*provision.PlacementIntent, error) {
	err := network.Validate()
	if err != nil {
		return nil, err
	}
	builder := newPostgresPlacementBuilder(network)
	switch t := template.GetTemplate().(type) {
	case *database.Database_Template_PostgresInstance:
		return builder.BuildForPostgresInstance(t)
	case *database.Database_Template_PostgresCluster:
		return builder.BuildForPostgresCluster(t)
	default:
		return nil, fmt.Errorf("unknown database template type")
	}
}

func (p ProvisionerService) getStroppyWorkerIp(network *deployment.Network) (*deployment.Ip, error) {
	_, cidr, err := net.ParseCIDR(network.GetCidr().GetValue())
	if err != nil {
		return nil, err
	}
	ip, err := ips.FirstFreeIP(cidr, nil)
	if err != nil {
		return nil, err
	}
	return &deployment.Ip{
		Value: ip.String(),
	}, nil
}

func (p ProvisionerService) getCloudInitForEdgeWorker(
	workerName string,
	vmUser *deployment.VmUser,
	acceptableTasks []*edge.Task_Identifier,
) (*deployment.CloudInit, error) {
	return scripting.InstallEdgeWorkerCloudInit(
		scripting.WithUser(vmUser),
		scripting.WithEnv(map[string]string{
			edgeDomain.WorkerNameEnvKey:            workerName,
			edgeDomain.WorkerAcceptableTasksEnvKey: edgeDomain.TaskIdListToString(acceptableTasks),
			HatchetServerUrlKey:                    os.Getenv(HatchetServerUrlKey),
			HatchetServerHostPortKey:               os.Getenv(HatchetServerHostPortKey),
			HatchetClientTokenKey:                  os.Getenv(HatchetClientTokenKey),
			HatchetClientTlsStrategyKey:            HatchetClientTlsStrategyNone,
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
	)
}

func (p ProvisionerService) BuildPlacement(
	_ context.Context,
	testRunCtx *stroppy.TestRunContext,
	intent *provision.PlacementIntent,
) (*provision.Placement, error) {
	var items []*provision.Placement_Item
	runId := ids.ParseRunId(testRunCtx.GetRunId())
	for _, item := range intent.GetItems() {
		workerName := edgeDomain.NewWorkerName(runId, item.GetName())
		workerAcceptableTasks := []*edge.Task_Identifier{
			edgeDomain.NewTaskId(runId, edge.Task_KIND_SETUP_CONTAINERS),
			edgeDomain.NewTaskId(runId, edge.Task_KIND_RUN_STROPPY),
		}
		metadata := lo.Assign(
			item.GetMetadata(),
			databaseMetadata(testRunCtx.GetRunId()),
		)
		worker := &edge.Worker{
			WorkerName:      workerName,
			AcceptableTasks: workerAcceptableTasks,
			Metadata:        metadata,
		}
		workerCloudInit, err := p.getCloudInitForEdgeWorker(
			workerName,
			// TODO: dispatch by cloud
			testRunCtx.GetSelectedTarget().GetYandexCloudSettings().GetVmSettings().GetVmUser(),
			workerAcceptableTasks,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, &provision.Placement_Item{
			Name:       item.GetName(),
			Containers: item.GetContainers(),
			VmTemplate: &deployment.Vm_Template{
				Identifier: &deployment.Identifier{
					Id:     ids.NewUlid().Lower().String(),
					Name:   workerName,
					Target: getDeploymentTarget(testRunCtx.GetSelectedTarget()),
				},
				Hardware: item.GetHardware(),
				// TODO: dispatch by cloud
				BaseImageId: testRunCtx.GetSelectedTarget().GetYandexCloudSettings().GetVmSettings().GetBaseImageId(),
				HasPublicIp: testRunCtx.GetSelectedTarget().GetYandexCloudSettings().GetVmSettings().GetEnablePublicIps(),
				VmUser:      testRunCtx.GetSelectedTarget().GetYandexCloudSettings().GetVmSettings().GetVmUser(),
				InternalIp:  item.GetInternalIp(),
				CloudInit:   workerCloudInit,
				Labels:      metadata,
			},
			Worker:   worker,
			Metadata: metadata,
		})
	}

	stroppyWorkerName := edgeDomain.NewWorkerName(runId, metadataRoleStroppyValue)
	stroppyWorkerIp, err := p.getStroppyWorkerIp(intent.GetNetwork())
	if err != nil {
		return nil, err
	}
	stroppyWorkerAcceptableTasks := []*edge.Task_Identifier{
		edgeDomain.NewTaskId(runId, edge.Task_KIND_SETUP_CONTAINERS),
		edgeDomain.NewTaskId(runId, edge.Task_KIND_INSTALL_STROPPY),
		edgeDomain.NewTaskId(runId, edge.Task_KIND_RUN_STROPPY),
	}
	stroppyCloudInit, err := p.getCloudInitForEdgeWorker(
		stroppyWorkerName,
		// TODO: dispatch by cloud
		testRunCtx.GetSelectedTarget().GetYandexCloudSettings().GetVmSettings().GetVmUser(),
		stroppyWorkerAcceptableTasks,
	)
	if err != nil {
		return nil, err
	}
	stroppyMd := stroppyMetadata(testRunCtx.GetRunId())
	return &provision.Placement{
		Network:          intent.GetNetwork(),
		ConnectionString: intent.GetConnectionString(),
		DeploymentTemplate: &deployment.Deployment_Template{
			Identifier: &deployment.Identifier{
				Id:     ids.NewUlid().Lower().String(),
				Name:   globalDeploymentName,
				Target: getDeploymentTarget(testRunCtx.GetSelectedTarget()),
			},
			Network:     intent.GetNetwork(),
			VmTemplates: make([]*deployment.Vm_Template, 0),
			Metadata:    stroppyMd,
		},
		Items: append(items, &provision.Placement_Item{
			Name:       metadataRoleStroppyValue,
			Containers: []*provision.Container{},
			VmTemplate: &deployment.Vm_Template{
				Identifier: &deployment.Identifier{
					Id:     ids.NewUlid().Lower().String(),
					Name:   stroppyWorkerName,
					Target: getDeploymentTarget(testRunCtx.GetSelectedTarget()),
				},
				Hardware: testRunCtx.GetTest().GetStroppyHardware(),
				// TODO: dispatch by cloud
				BaseImageId: testRunCtx.GetSelectedTarget().GetYandexCloudSettings().GetVmSettings().GetBaseImageId(),
				HasPublicIp: testRunCtx.GetSelectedTarget().GetYandexCloudSettings().GetVmSettings().GetEnablePublicIps(),
				VmUser:      testRunCtx.GetSelectedTarget().GetYandexCloudSettings().GetVmSettings().GetVmUser(),
				InternalIp:  stroppyWorkerIp,
				CloudInit:   stroppyCloudInit,
				Labels:      stroppyMd,
			},
			Worker: &edge.Worker{
				WorkerName:      stroppyWorkerName,
				AcceptableTasks: stroppyWorkerAcceptableTasks,
				Metadata:        stroppyMd,
			},
			Metadata: stroppyMd,
		}),
	}, nil
}

func (p ProvisionerService) DeployPlan(
	ctx context.Context,
	placement *provision.Placement,
) (*provision.DeployedPlacement, error) {
	depl, err := p.deploymentService.CreateDeployment(ctx, placement.GetDeploymentTemplate())
	if err != nil {
		return nil, err
	}
	var deployedItems []*provision.DeployedPlacement_Item
	for _, item := range placement.GetItems() {
		vm, ok := lo.Find(depl.GetVms(), func(i *deployment.Vm) bool {
			return i.GetTemplate().GetIdentifier().GetId() == item.GetVmTemplate().GetIdentifier().GetId()
		})
		if !ok {
			return nil, fmt.Errorf(
				"vm instance not found for vm template %s",
				item.GetVmTemplate().GetIdentifier().GetId(),
			)
		}
		deployedItems = append(deployedItems, &provision.DeployedPlacement_Item{
			PlacementItem: item,
			Vm:            vm,
		})
	}
	return &provision.DeployedPlacement{
		Items:            deployedItems,
		Deployment:       depl,
		Network:          placement.GetNetwork(),
		ConnectionString: placement.GetConnectionString(),
	}, nil
}

func (p ProvisionerService) DestroyPlan(
	ctx context.Context,
	deployedPlacement *provision.DeployedPlacement,
) error {
	if err := p.deploymentService.DestroyDeployment(ctx, deployedPlacement.GetDeployment()); err != nil {
		return err
	}
	return p.networkManager.FreeNetwork(ctx, deployedPlacement.GetNetwork())
}
