package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// stroppyMachinesOutputKey is the terraform output every provider module
// must export (spec §3): a list of {id, private_ip, public_ip?} describing
// the machines it provisioned for the stroppy_nodes it was given.
const stroppyMachinesOutputKey = "stroppy_machines"

// terraformProvider adapts a terraform module (moduleDir) into the Provider
// interface: it lowers MachineGroups into the module's stroppy_nodes input,
// applies it via exec, and lowers the stroppy_machines output back into
// MachineState.
type terraformProvider struct {
	moduleDir string
	exec      terraformExec
}

// NewTerraform builds a Provider backed by a terraform module living at
// moduleDir. exec performs the actual apply/destroy (see terraformExec for
// the adaptation gap against the real terraform.Actor runner).
func NewTerraform(moduleDir string, exec terraformExec) Provider {
	return &terraformProvider{moduleDir: moduleDir, exec: exec}
}

// tfNodeDisk is the `disk` field of a stroppy_nodes entry.
type tfNodeDisk struct {
	SizeGB uint64 `json:"size_gb"`
	Type   string `json:"type"`
}

// tfNode is one entry of the module's stroppy_nodes input variable.
type tfNode struct {
	ID    string         `json:"id"`
	Group string         `json:"group"`
	CPU   uint32         `json:"cpu"`
	RamGB float64        `json:"ram_gb"`
	Disk  *tfNodeDisk    `json:"disk,omitempty"`
	Ext   map[string]any `json:"ext"`
}

// tfMachineOutput is one entry of the module's stroppy_machines output.
type tfMachineOutput struct {
	ID        string `json:"id"`
	PrivateIP string `json:"private_ip"`
	PublicIP  string `json:"public_ip"`
}

func (p *terraformProvider) Provision(ctx context.Context, ref *dslpb.ProviderRef, groups []*dslpb.MachineGroup) (map[string][]*deploymentpb.MachineState, error) {
	params, err := decodeParams(ref)
	if err != nil {
		return nil, err
	}

	nodesByID := make(map[string]string, len(groups)) // node id -> group name
	nodes := make([]tfNode, 0, len(groups))
	for _, group := range groups {
		groupNodes, err := tfNodesForGroup(group)
		if err != nil {
			return nil, err
		}
		for _, node := range groupNodes {
			nodesByID[node.ID] = group.GetName()
		}
		nodes = append(nodes, groupNodes...)
	}
	params["stroppy_nodes"] = nodes

	varsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal terraform tfvars: %w", err)
	}

	outputs, err := p.exec.Apply(ctx, p.moduleDir, varsJSON)
	if err != nil {
		return nil, fmt.Errorf("terraform apply: %w", err)
	}

	raw, ok := outputs[stroppyMachinesOutputKey]
	if !ok {
		return nil, fmt.Errorf("terraform module must export %s output", stroppyMachinesOutputKey)
	}

	var machines []tfMachineOutput
	if err := json.Unmarshal(raw, &machines); err != nil {
		return nil, fmt.Errorf("decode %s output: %w", stroppyMachinesOutputKey, err)
	}

	result := make(map[string][]*deploymentpb.MachineState, len(groups))
	seen := make(map[string]int, len(nodesByID)) // group -> count of machines seen
	for _, machine := range machines {
		groupName, ok := nodesByID[machine.ID]
		if !ok {
			return nil, fmt.Errorf("%s output: machine id %q does not match any requested node", stroppyMachinesOutputKey, machine.ID)
		}
		seen[groupName]++
		result[groupName] = append(result[groupName], machineState(groupName, machine))
	}

	for _, group := range groups {
		want := int(group.GetCount())
		got := seen[group.GetName()]
		if got != want {
			return nil, fmt.Errorf("%s output: group %q expected %d machines, got %d", stroppyMachinesOutputKey, group.GetName(), want, got)
		}
	}

	return result, nil
}

func (p *terraformProvider) Destroy(ctx context.Context, ref *dslpb.ProviderRef) error {
	params, err := decodeParams(ref)
	if err != nil {
		return err
	}
	varsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal terraform tfvars: %w", err)
	}
	if err := p.exec.Destroy(ctx, p.moduleDir, varsJSON); err != nil {
		return fmt.Errorf("terraform destroy: %w", err)
	}
	return nil
}

// decodeParams unmarshals ref.ParamsJson into a fresh map so provider params
// can be spread at the tfvars top level alongside stroppy_nodes. An empty
// ParamsJson yields an empty (non-nil) map.
func decodeParams(ref *dslpb.ProviderRef) (map[string]any, error) {
	params := map[string]any{}
	raw := ref.GetParamsJson()
	if raw == "" {
		return params, nil
	}
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return nil, fmt.Errorf("decode provider params_json: %w", err)
	}
	return params, nil
}

// tfNodesForGroup expands one MachineGroup into its "<group>-<idx>"
// stroppy_nodes entries.
func tfNodesForGroup(group *dslpb.MachineGroup) ([]tfNode, error) {
	ext := map[string]any{}
	if raw := group.GetExtJson(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &ext); err != nil {
			return nil, fmt.Errorf("group %q: decode ext_json: %w", group.GetName(), err)
		}
	}

	var disk *tfNodeDisk
	if disks := group.GetDisks(); len(disks) > 0 {
		disk = &tfNodeDisk{
			SizeGB: disks[0].GetSizeGb(),
			Type:   strings.ToLower(disks[0].GetType()),
		}
	}

	nodes := make([]tfNode, group.GetCount())
	for idx := range nodes {
		nodes[idx] = tfNode{
			ID:    fmt.Sprintf("%s-%d", group.GetName(), idx),
			Group: group.GetName(),
			CPU:   group.GetCpu(),
			RamGB: float64(group.GetRamMb()) / 1024,
			Disk:  disk,
			Ext:   ext,
		}
	}
	return nodes, nil
}

// machineState lowers one stroppy_machines output entry into a MachineState,
// mirroring the private/public endpoint convention used elsewhere (see
// internal/workflows/deployment.go's privateEndpoints).
func machineState(groupName string, machine tfMachineOutput) *deploymentpb.MachineState {
	endpoints := []*deploymentpb.Endpoint{
		{
			Name:    "private",
			Address: machine.PrivateIP,
			Labels:  map[string]string{"scope": "private"},
		},
	}
	if machine.PublicIP != "" {
		endpoints = append(endpoints, &deploymentpb.Endpoint{
			Name:    "public",
			Address: machine.PublicIP,
			Labels:  map[string]string{"scope": "public"},
		})
	}
	return &deploymentpb.MachineState{
		NodeId:             machine.ID,
		ProviderResourceId: machine.ID,
		Status:             common.Status_STATUS_DEPLOYED,
		Endpoints:          endpoints,
		Labels: map[string]string{
			"node_id": machine.ID,
			"group":   groupName,
		},
	}
}
