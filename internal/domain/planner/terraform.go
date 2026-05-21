package planner

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/deployments/terraform/yandex"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

// buildTfOperation builds the TfOperation for an apply/destroy node: the embedded
// Yandex module + a terraform.tfvars.json derived from the topology. apply and
// destroy share the same workdir_id (crash-safe teardown of the same state).
func buildTfOperation(action ops.TfOperation_Action, workdirID string, preset *domain.TestPreset, params *DeploymentParams) (*ops.TfOperation, error) {
	files, err := yandex.EmbeddedFiles()
	if err != nil {
		return nil, fmt.Errorf("planner: embed tf module: %w", err)
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
// TODO(planner): per-vm internal_ip + user_data depend on the subnet network
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
		return nil, fmt.Errorf("planner: marshal tfvars: %w", err)
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
