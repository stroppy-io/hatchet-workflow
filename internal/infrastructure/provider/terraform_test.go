package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// fakeTerraformExec is a test double for terraformExec: it captures every
// Apply/Destroy call's (dir, varsJSON) and returns a scripted outputs map
// (or error) for Apply.
type fakeTerraformExec struct {
	applyDir     string
	applyVars    []byte
	applyOutputs map[string][]byte
	applyErr     error

	destroyDir  string
	destroyVars []byte
	destroyErr  error
}

func (f *fakeTerraformExec) Apply(_ context.Context, dir string, varsJSON []byte) (map[string][]byte, error) {
	f.applyDir = dir
	f.applyVars = varsJSON
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	return f.applyOutputs, nil
}

func (f *fakeTerraformExec) Destroy(_ context.Context, dir string, varsJSON []byte) error {
	f.destroyDir = dir
	f.destroyVars = varsJSON
	return f.destroyErr
}

func dbGroup() *dslpb.MachineGroup {
	return &dslpb.MachineGroup{
		Name:  "db",
		Count: 3,
		Cpu:   8,
		RamMb: 32 * 1024,
		Disks: []*dslpb.DiskSpec{
			{SizeGb: 100, Type: "NETWORK-SSD"},
		},
		ExtJson: `{"platform_id":"standard-v3"}`,
	}
}

func stroppyMachinesOutput(t *testing.T, machines []map[string]any) map[string][]byte {
	t.Helper()
	raw, err := json.Marshal(machines)
	require.NoError(t, err)
	return map[string][]byte{"stroppy_machines": raw}
}

func TestTerraform_Provision_BuildsTfvarsAndParsesMachines(t *testing.T) {
	fake := &fakeTerraformExec{
		applyOutputs: stroppyMachinesOutput(t, []map[string]any{
			{"id": "db-0", "private_ip": "10.0.0.1", "public_ip": "1.2.3.4"},
			{"id": "db-1", "private_ip": "10.0.0.2"},
			{"id": "db-2", "private_ip": "10.0.0.3"},
		}),
	}
	p := NewTerraform("/modules/yandex", fake)

	ref := &dslpb.ProviderRef{Name: "yandex", ParamsJson: `{"zone":"ru-central1-a"}`}
	groups := []*dslpb.MachineGroup{dbGroup()}

	result, err := p.Provision(context.Background(), ref, groups)
	require.NoError(t, err)

	// --- tfvars sent to the module ---
	require.Equal(t, "/modules/yandex", fake.applyDir)

	var tfvars map[string]any
	require.NoError(t, json.Unmarshal(fake.applyVars, &tfvars))

	require.Equal(t, "ru-central1-a", tfvars["zone"], "provider params must be spread at tfvars top level")

	nodesRaw, ok := tfvars["stroppy_nodes"]
	require.True(t, ok, "tfvars must contain stroppy_nodes")
	nodes, ok := nodesRaw.([]any)
	require.True(t, ok)
	require.Len(t, nodes, 3)

	for idx, raw := range nodes {
		node, ok := raw.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "db-"+itoa(idx), node["id"])
		require.Equal(t, "db", node["group"])
		require.InDelta(t, 8, node["cpu"], 0)
		require.InDelta(t, 32, node["ram_gb"], 0)

		disk, ok := node["disk"].(map[string]any)
		require.True(t, ok, "disk must be present")
		require.InDelta(t, 100, disk["size_gb"], 0)
		require.Equal(t, "network-ssd", disk["type"], "disk type must be lowered")

		ext, ok := node["ext"].(map[string]any)
		require.True(t, ok, "ext must be present")
		require.Equal(t, "standard-v3", ext["platform_id"], "ext round-trips from ext_json")
	}

	// --- parsed MachineState grouping ---
	require.Contains(t, result, "db")
	require.Len(t, result["db"], 3)

	byNodeID := map[string]string{}
	for _, m := range result["db"] {
		byNodeID[m.GetNodeId()] = privateAddress(m)
	}
	require.Equal(t, "10.0.0.1", byNodeID["db-0"])
	require.Equal(t, "10.0.0.2", byNodeID["db-1"])
	require.Equal(t, "10.0.0.3", byNodeID["db-2"])

	var db0 *deploymentpb.MachineState
	for _, m := range result["db"] {
		if m.GetNodeId() == "db-0" {
			db0 = m
		}
	}
	require.NotNil(t, db0)
	require.Equal(t, "db", db0.GetLabels()["group"])
	foundPublic := false
	for _, ep := range db0.GetEndpoints() {
		if ep.GetName() == "public" {
			foundPublic = true
			require.Equal(t, "1.2.3.4", ep.GetAddress())
		}
	}
	require.True(t, foundPublic, "db-0 must carry a public endpoint")
}

func privateAddress(m *deploymentpb.MachineState) string {
	for _, ep := range m.GetEndpoints() {
		if ep.GetName() == "private" {
			return ep.GetAddress()
		}
	}
	return ""
}

func itoa(i int) string {
	digits := "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{digits[i%10]}, b...)
		i /= 10
	}
	return string(b)
}

func TestTerraform_Provision_MissingStroppyMachinesOutput(t *testing.T) {
	fake := &fakeTerraformExec{applyOutputs: map[string][]byte{}}
	p := NewTerraform("/modules/yandex", fake)

	ref := &dslpb.ProviderRef{Name: "yandex", ParamsJson: `{"zone":"ru-central1-a"}`}
	groups := []*dslpb.MachineGroup{dbGroup()}

	_, err := p.Provision(context.Background(), ref, groups)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "stroppy_machines"))
}

// privateLabel returns a label from a MachineState's "private" endpoint, or
// "" if the endpoint or label is absent.
func privateLabel(m *deploymentpb.MachineState, key string) string {
	for _, ep := range m.GetEndpoints() {
		if ep.GetName() == "private" {
			return ep.GetLabels()[key]
		}
	}
	return ""
}

// TestTerraform_Provision_StampsDiskDeviceLabel is the I2 provider-side lock:
// machineState must carry the disk_device label on the private endpoint —
// from the module output when present, defaulting to defaultDiskDevice
// ("/dev/vdb") when the module omits it.
func TestTerraform_Provision_StampsDiskDeviceLabel(t *testing.T) {
	fake := &fakeTerraformExec{
		applyOutputs: stroppyMachinesOutput(t, []map[string]any{
			{"id": "db-0", "private_ip": "10.0.0.1", "disk_device": "/dev/nvme1n1"},
			{"id": "db-1", "private_ip": "10.0.0.2"},
		}),
	}
	p := NewTerraform("/modules/yandex", fake)

	ref := &dslpb.ProviderRef{Name: "yandex", ParamsJson: `{"zone":"ru-central1-a"}`}
	group := dbGroup()
	group.Count = 2
	groups := []*dslpb.MachineGroup{group}

	result, err := p.Provision(context.Background(), ref, groups)
	require.NoError(t, err)

	byNodeID := map[string]*deploymentpb.MachineState{}
	for _, m := range result["db"] {
		byNodeID[m.GetNodeId()] = m
	}

	require.Equal(t, "/dev/nvme1n1", privateLabel(byNodeID["db-0"], "disk_device"),
		"module-provided disk_device must be threaded through")
	require.Equal(t, defaultDiskDevice, privateLabel(byNodeID["db-1"], "disk_device"),
		"disk_device must default to defaultDiskDevice when the module output omits it")
}

func TestTerraform_Destroy_DelegatesWithSameParams(t *testing.T) {
	fake := &fakeTerraformExec{}
	p := NewTerraform("/modules/yandex", fake)

	ref := &dslpb.ProviderRef{Name: "yandex", ParamsJson: `{"zone":"ru-central1-a"}`}
	err := p.Destroy(context.Background(), ref)
	require.NoError(t, err)

	require.Equal(t, "/modules/yandex", fake.destroyDir)
	var tfvars map[string]any
	require.NoError(t, json.Unmarshal(fake.destroyVars, &tfvars))
	require.Equal(t, "ru-central1-a", tfvars["zone"])
}
