//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/planner"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
)

// TestPGHABindingsResolveOnRealNetwork validates the HA keystone (H27): a compiled
// PG-HA topology (etcd + 2 patroni databases) renders per-component configs whose
// late-binding holes resolve to the REAL private IPs of containers on a shared
// docker network — patroni.yml gets the etcd hosts + its own connect address, etc.
// — with no token left unresolved.
func TestPGHABindingsResolveOnRealNetwork(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = net.Remove(ctx) })

	// One container per machine; collect each component's private IP on the network.
	machines := []string{"m-etcd", "m-db1", "m-db2"}
	componentMachine := map[string]string{"etcd1": "m-etcd", "db1": "m-db1", "db2": "m-db2"}
	machineIP := map[string]string{}
	for _, m := range machines {
		c, cerr := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:      "alpine:3.20",
				Cmd:        []string{"sleep", "600"},
				Networks:   []string{net.Name},
				WaitingFor: wait.ForExec([]string{"true"}),
			},
			Started: true,
		})
		require.NoError(t, cerr, "machine %s", m)
		t.Cleanup(func() { _ = c.Terminate(ctx) })
		ips, ierr := c.ContainerIPs(ctx)
		require.NoError(t, ierr)
		require.NotEmpty(t, ips)
		machineIP[m] = ips[len(ips)-1] // the custom-network IP
	}

	resolved := render.Resolved{}
	for comp, m := range componentMachine {
		resolved[comp] = map[string]string{render.AttrPrivateIP: machineIP[m], render.AttrEndpoint: machineIP[m]}
	}
	resolver := render.NewResolver(resolved)

	preset := &domain.TestPreset{
		Database: &domain.Database{
			Kind: domain.Database_KIND_POSTGRES, Version: "16",
			Options: &domain.Database_Options{Options: &domain.Database_Options_Postgres_{Postgres: &domain.Database_Options_Postgres{
				Replication: &domain.Database_Options_Postgres_Replication{Mode: domain.Database_Options_Postgres_Replication_MODE_PATRONI},
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				{Id: "m-etcd", Cores: 2, MemoryGb: 4, Components: []*domain.Topology_Component{{Id: "etcd1", Kind: domain.Topology_Component_KIND_COORDINATOR}}},
				{Id: "m-db1", Cores: 4, MemoryGb: 8, Components: []*domain.Topology_Component{{Id: "db1", Kind: domain.Topology_Component_KIND_DATABASE}}},
				{Id: "m-db2", Cores: 4, MemoryGb: 8, Components: []*domain.Topology_Component{{Id: "db2", Kind: domain.Topology_Component_KIND_DATABASE}}},
			},
			Connections: []*domain.Topology_Connection{
				{From: "db1", To: "etcd1", Kind: domain.Topology_Connection_KIND_COORDINATION},
				{From: "db2", To: "etcd1", Kind: domain.Topology_Connection_KIND_COORDINATION},
				{From: "db1", To: "db2", Kind: domain.Topology_Connection_KIND_REPLICATION},
			},
		},
	}
	dag, err := planner.New().Compile(preset, nil)
	require.NoError(t, err)

	sub := findSubDag(t, dag, "install_and_run")
	checked := 0
	for _, n := range sub.GetNodes() {
		var cmd rtagent.Command
		if uerr := n.GetTaskState().GetInput().UnmarshalTo(&cmd); uerr != nil {
			continue
		}
		wf := cmd.GetOperation().GetWriteFile()
		if wf == nil {
			continue
		}
		bindings, berr := render.NodeBindings(n)
		require.NoError(t, berr)
		out, rerr := resolver.ResolveText(wf.GetContent().GetText(), bindings)
		require.NoErrorf(t, rerr, "resolve %s", n.GetId())
		require.NotContainsf(t, out, "__", "node %s left an unresolved binding token:\n%s", n.GetId(), out)
		checked++

		// patroni.yml on db1 must point at etcd1's real IP and its own self IP.
		if n.GetId() == "db1.write_patroni.yml" {
			require.Contains(t, out, machineIP["m-etcd"]+":2379", "patroni etcd host")
			require.Contains(t, out, machineIP["m-db1"]+":8008", "patroni self connect_address")
		}
		// etcd config on etcd1 must list its own peer at the resolved self IP.
		if n.GetId() == "etcd1.write_etcd-default" {
			require.Contains(t, out, machineIP["m-etcd"], "etcd advertise IP")
		}
	}
	require.Greater(t, checked, 0, "no WRITE_FILE nodes with bindings were checked")
}

func findSubDag(t *testing.T, dag *primitive.Dag, id string) *primitive.Dag {
	t.Helper()
	for _, n := range dag.GetNodes() {
		if n.GetId() == id {
			return n.GetSubDag()
		}
	}
	t.Fatalf("sub-dag %q not found", id)
	return nil
}
