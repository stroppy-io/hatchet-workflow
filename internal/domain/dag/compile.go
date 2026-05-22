package dag

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/deployments/terraform/yandex"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

// Server-locus task handlers (run by the control-plane executor's TasksRegistry).
const (
	HandlerRenderConfig     = "render_config"
	HandlerTerraformApply   = "terraform_apply"
	HandlerTerraformDestroy = "terraform_destroy"
	HandlerCollectResults   = "collect_results"
)

// Compiler compiles a TestPreset's topology graph into a Dag. Cluster shapes
// (single / ha / replica / scale) are NOT enumerated — they emerge structurally
// from the graph (component kinds + connections + Database.Options), so one
// graph-driven template handles every shape (topology.proto C-note).
//
// TODO(dag): this is the LEGACY linear terraform-only compile path
// (render_config -> terraform_apply -> install_and_run -> collect_results ->
// terraform_destroy). The philosophy target is BuildTestDag (one static proto Dag
// with conditional provider edges over a proto deployment.Deployment); the
// run/suite services still consume Compile, so it stays as a thin shim around the
// install sub-dag + terraform nodes until they migrate to BuildTestDag. When they
// do, drop DeploymentParams in favour of the proto deployment.Deployment built by
// BuildDeployment.
type Compiler struct{}

// New builds a Compiler.
func New() *Compiler { return &Compiler{} }

// Compile builds the Dag for a preset with resolved deployment params (nil = empty).
func (c *Compiler) Compile(preset *domain.TestPreset, params *DeploymentParams) (*primitive.Dag, error) {
	if params == nil {
		params = &DeploymentParams{}
	}
	return graphTemplate(preset, params)
}

// graphTemplate: render_config -> terraform_apply -> install_and_run(sub_dag) ->
// collect_results -> terraform_destroy(always_run). apply/destroy share a workdir.
func graphTemplate(preset *domain.TestPreset, params *DeploymentParams) (*primitive.Dag, error) {
	workdirID := ids.New()

	applyOp, err := buildTfOperation(ops.TfOperation_ACTION_APPLY, workdirID, preset, params)
	if err != nil {
		return nil, err
	}
	applyInput, err := anypb.New(applyOp)
	if err != nil {
		return nil, fmt.Errorf("dag: wrap apply op: %w", err)
	}
	destroyOp, err := buildTfOperation(ops.TfOperation_ACTION_DESTROY, workdirID, preset, params)
	if err != nil {
		return nil, err
	}
	destroyInput, err := anypb.New(destroyOp)
	if err != nil {
		return nil, fmt.Errorf("dag: wrap destroy op: %w", err)
	}

	view := viewTopology(preset.GetTopology())

	renderNode := serverTask("render_config", HandlerRenderConfig)
	apply := serverTaskInput("terraform_apply", HandlerTerraformApply, applyInput)
	installSub, err := BuildInstallDag(preset)
	if err != nil {
		return nil, err
	}
	install := legacySubDagNode("install_and_run", installSub)
	collect := serverTask("collect_results", HandlerCollectResults)
	destroy := serverTaskInput("terraform_destroy", HandlerTerraformDestroy, destroyInput)
	destroy.Scheduling.AlwaysRun = true // teardown runs on failure/cancel too (H58)

	return &primitive.Dag{
		Id:     ids.New(),
		Status: primitive.Status_STATUS_PENDING,
		Nodes:  []*primitive.Dag_Node{renderNode, apply, install, collect, destroy},
		Edges: []*primitive.Dag_Edge{
			chainEdge(renderNode, apply),
			chainEdge(apply, install),
			chainEdge(install, collect),
			chainEdge(collect, destroy), // success path; always_run covers failure
		},
		// component->machine lets the execute-seam resolver map binding component
		// ids to terraform vm outputs.
		Metadata: map[string]string{
			render.ComponentMachineMetaKey: render.EncodeComponentMachine(view.componentToMachine),
		},
	}, nil
}

// serverTaskInput is a server task node carrying a typed input payload.
func serverTaskInput(id, handler string, input *anypb.Any) *primitive.Dag_Node {
	node := serverTask(id, handler)
	node.GetTaskState().Input = input
	return node
}

func serverTask(id, handler string) *primitive.Dag_Node {
	return &primitive.Dag_Node{
		Id:          id,
		ExecutionId: id,
		Status:      primitive.Status_STATUS_PENDING,
		Scheduling:  &primitive.Dag_Node_Scheduling{},
		Variant: &primitive.Dag_Node_TaskState_{TaskState: &primitive.Dag_Node_TaskState{
			HandlerName: handler,
			Locus:       primitive.Dag_Node_TaskState_EXECUTION_LOCUS_SERVER,
		}},
	}
}

// legacySubDagNode wraps a sub-dag for the legacy linear Compile path.
func legacySubDagNode(id string, sub *primitive.Dag) *primitive.Dag_Node {
	return &primitive.Dag_Node{
		Id:          id,
		ExecutionId: id,
		Status:      primitive.Status_STATUS_PENDING,
		Scheduling:  &primitive.Dag_Node_Scheduling{},
		Variant:     &primitive.Dag_Node_SubDag{SubDag: sub},
	}
}

// ── DeploymentParams (terraform-vars carrier for the legacy Compile path) ───────
//
// TODO(dag): DeploymentParams is a Go struct that deviates from the proto-first
// philosophy (the model should be the proto deployment.Deployment, built by
// BuildDeployment). It survives because the run/suite/provider/deploy chain still
// flows it into Compile; replacing it cascades through provider.Resolver +
// deploy.Resolver. Migrate those to deployment.Deployment when Compile is retired
// in favour of BuildTestDag.

// DeploymentParams carries the resolved, provider-specific values the planner
// needs to render terraform vars. Closed-set values are typed deployment.Yandex
// enums (translated to terraform strings only at marshal, via the yandex module's
// mappers); free identifiers stay strings. Pure data — the planner stays I/O-free.
// The caller (RunService) builds it from tenant provider settings (SettingsService),
// the subnet network allocation (D20), and the agent cloud-init bootstrap (D18).
type DeploymentParams struct {
	Platform            deployment.Yandex_PlatformId
	Zone                deployment.Yandex_Zone
	BootDiskType        deployment.Yandex_DiskType
	NetworkAcceleration deployment.Yandex_NetworkAcceleration

	ImageID        string
	NetworkID      string
	NetworkName    string
	NetworkCIDR    string
	AssignPublicIP bool

	// Per-machine values (machine id -> value).
	//
	// TODO(dag): MachineInternalIP comes from the subnet network allocation
	// (D20); MachineUserData is the agent cloud-init JWT (D18). Empty until wired.
	MachineInternalIP map[string]string
	MachineUserData   map[string]string
}

// bootDiskType returns the disk type, defaulting to network-ssd when unset.
func (p *DeploymentParams) bootDiskType() deployment.Yandex_DiskType {
	if p == nil || p.BootDiskType == deployment.Yandex_DISK_TYPE_UNSPECIFIED {
		return deployment.Yandex_DISK_TYPE_NETWORK_SSD
	}
	return p.BootDiskType
}

// networkAcceleration returns the acceleration mode, defaulting to standard.
func (p *DeploymentParams) networkAcceleration() deployment.Yandex_NetworkAcceleration {
	if p == nil || p.NetworkAcceleration == deployment.Yandex_NETWORK_ACCELERATION_UNSPECIFIED {
		return deployment.Yandex_NETWORK_ACCELERATION_STANDARD
	}
	return p.NetworkAcceleration
}

func (p *DeploymentParams) internalIP(machineID string) string {
	if p == nil {
		return ""
	}
	return p.MachineInternalIP[machineID]
}

func (p *DeploymentParams) userData(machineID string) string {
	if p == nil {
		return ""
	}
	return p.MachineUserData[machineID]
}

// ── terraform tfvars rendering (legacy Compile path) ────────────────────────────

// buildTfOperation builds the TfOperation for an apply/destroy node: the embedded
// Yandex module + a terraform.tfvars.json derived from the topology. apply and
// destroy share the same workdir_id (crash-safe teardown of the same state).
func buildTfOperation(action ops.TfOperation_Action, workdirID string, preset *domain.TestPreset, params *DeploymentParams) (*ops.TfOperation, error) {
	files, err := yandex.EmbeddedFiles()
	if err != nil {
		return nil, fmt.Errorf("dag: embed tf module: %w", err)
	}
	varFile, err := tfvarsFile(preset.GetTopology(), params)
	if err != nil {
		return nil, err
	}
	return &ops.TfOperation{Input: &ops.TfOperation_Input{
		Action:    action,
		WorkdirId: workdirID,
		Files:     files,
		VarFile:   varFile,
	}}, nil
}

// tfvarsFile renders terraform.tfvars.json: compute.vms sized from topology
// machines, network + provider + per-vm placement from the resolved
// DeploymentParams.
//
// TODO(dag): per-vm internal_ip + user_data depend on the subnet network
// allocation (D20) and the agent cloud-init JWT (D18) — empty in params until
// those land, so terraform validation will fail for a real apply.
func tfvarsFile(topo *domain.Topology, params *DeploymentParams) (*system.File, error) {
	// Closed-set enums -> terraform strings via the module's canonical mappers.
	zone := yandex.ZoneString(params.Zone)
	diskType := yandex.DiskTypeString(params.bootDiskType())
	accel := yandex.NetworkAccelerationString(params.networkAcceleration())

	vms := make(map[string]*deployment.Yandex_Vm, len(topo.GetMachines()))
	for _, m := range topo.GetMachines() {
		disks := make([]*deployment.Yandex_Disk, 0, len(m.GetDataDisksGb()))
		for i, gb := range m.GetDataDisksGb() {
			disks = append(disks, &deployment.Yandex_Disk{
				DeviceName: fmt.Sprintf("data-%d", i),
				SizeGb:     uint32(gb),
				Type:       diskType,
			})
		}
		vms[m.GetId()] = &deployment.Yandex_Vm{
			Cores:               m.GetCores(),
			MemoryGb:            m.GetMemoryGb(),
			BootDiskGb:          m.GetDiskGb(),
			BootDiskType:        diskType,
			Zone:                zone,
			InternalIp:          params.internalIP(m.GetId()),
			PublicIp:            params.AssignPublicIP,
			UserData:            params.userData(m.GetId()),
			NetworkAcceleration: accel,
			SecondaryDisks:      disks,
		}
	}
	// The Yandex.Input proto is the canonical tfvars shape: protojson with proto
	// field names yields terraform.tfvars.json directly (deployment/yandex.proto).
	input := &deployment.Yandex_Input{
		Network: &deployment.Yandex_Network{
			Name:      params.NetworkName,
			NetworkId: params.NetworkID,
			Cidr:      params.NetworkCIDR,
			Zone:      zone,
		},
		Compute: &deployment.Yandex_Compute{
			PlatformId: yandex.PlatformIDString(params.Platform),
			ImageId:    params.ImageID,
			Vms:        vms,
		},
	}
	data, err := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("dag: marshal tfvars: %w", err)
	}
	return textFile("terraform.tfvars.json", string(data)), nil
}

// textFile builds a workdir-relative inline-text system.File.
func textFile(path, content string) *system.File {
	return &system.File{
		Info: &system.File_Info{Path: path},
		Source: &system.File_Content_{Content: &system.File_Content{
			Content: &system.File_Content_Text{Text: content},
		}},
	}
}
