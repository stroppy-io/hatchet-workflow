//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/planner"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
)

// startSystemdHostOnNetwork starts a privileged systemd host joined to net.
func startSystemdHostOnNetwork(t *testing.T, ctx context.Context, netName string) (*systemdHost, string) {
	t.Helper()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      "jrei/systemd-ubuntu:22.04",
			Privileged: true,
			Networks:   []string{netName},
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.CgroupnsMode = "host"
				hc.Tmpfs = map[string]string{"/run": "exec,mode=755", "/run/lock": ""}
				hc.Binds = append(hc.Binds, "/sys/fs/cgroup:/sys/fs/cgroup:rw")
				hc.DNS = []string{"8.8.8.8", "1.1.1.1"}
			},
			WaitingFor: wait.ForExec([]string{"systemctl", "is-system-running", "--wait"}).
				WithExitCodeMatcher(func(code int) bool { return code == 0 || code == 1 }).
				WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "systemd host on network")
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	ips, err := c.ContainerIPs(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, ips)
	return &systemdHost{c: c}, ips[len(ips)-1]
}

// execOpResolved runs an op, first resolving its render-binding tokens against the
// component->ip resolver (HA configs/commands carry late-binding holes).
func (h *systemdHost) execOpResolved(t *testing.T, ctx context.Context, node *primitive.Dag_Node, op *ops.Operation, resolver *render.Resolver) {
	t.Helper()
	bindings, err := render.NodeBindings(node)
	require.NoError(t, err)

	switch v := op.GetOperation().(type) {
	case *ops.Operation_RunCmd:
		spec := v.RunCmd
		script := spec.GetScript().GetText()
		if script == "" {
			// argv form (e.g. run_stroppy) — join, resolve, run via shell.
			script = joinArgs(spec.GetArgv().GetArgs())
		}
		resolved, rerr := resolver.ResolveText(script, bindings)
		require.NoError(t, rerr)
		code, out := h.sh(t, ctx, resolved)
		require.Equalf(t, 0, code, "command failed: %s\n%s", resolved, out)
	case *ops.Operation_WriteFile:
		f := v.WriteFile
		resolved, rerr := resolver.ResolveText(f.GetContent().GetText(), bindings)
		require.NoError(t, rerr)
		path := f.GetInfo().GetPath()
		code, out := h.sh(t, ctx, "mkdir -p \"$(dirname '"+path+"')\" && cat > '"+path+"' <<'STROPPY_EOF'\n"+resolved+"\nSTROPPY_EOF")
		require.Equalf(t, 0, code, "write %s failed: %s", path, out)
	default:
		t.Fatalf("unsupported op %T", v)
	}
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

// TestPGHAFullPipeline brings up a real patroni HA cluster (1 etcd + 2 patroni
// postgres nodes) by executing the compiled dag's recipe ops across systemd hosts
// on a shared network, with bindings resolved to real container IPs, then asserts
// patroni elected a leader.
func TestPGHAFullPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = net.Remove(ctx) })

	componentMachine := map[string]string{"etcd1": "m-etcd", "db1": "m-db1", "db2": "m-db2"}
	hosts := map[string]*systemdHost{}
	machineIP := map[string]string{}
	for comp, m := range componentMachine {
		h, ip := startSystemdHostOnNetwork(t, ctx, net.Name)
		hosts[comp] = h
		machineIP[m] = ip
	}

	resolved := render.Resolved{}
	for comp, m := range componentMachine {
		resolved[comp] = map[string]string{render.AttrPrivateIP: machineIP[m], render.AttrEndpoint: machineIP[m]}
	}
	resolver := render.NewResolver(resolved)

	comp := func(id string, k domain.Topology_Component_Kind) *domain.Topology_Component {
		return &domain.Topology_Component{Id: id, Kind: k}
	}
	preset := &domain.TestPreset{
		Database: &domain.Database{
			Kind: domain.Database_KIND_POSTGRES, Version: "16",
			Options: &domain.Database_Options{Options: &domain.Database_Options_Postgres_{Postgres: &domain.Database_Options_Postgres{
				Replication: &domain.Database_Options_Postgres_Replication{Mode: domain.Database_Options_Postgres_Replication_MODE_PATRONI},
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				{Id: "m-etcd", Cores: 2, MemoryGb: 4, Components: []*domain.Topology_Component{comp("etcd1", domain.Topology_Component_KIND_COORDINATOR)}},
				{Id: "m-db1", Cores: 4, MemoryGb: 8, Components: []*domain.Topology_Component{comp("db1", domain.Topology_Component_KIND_DATABASE)}},
				{Id: "m-db2", Cores: 4, MemoryGb: 8, Components: []*domain.Topology_Component{comp("db2", domain.Topology_Component_KIND_DATABASE)}},
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

	// Run in rank order: etcd first, then both database nodes.
	for _, compID := range []string{"etcd1", "db1", "db2"} {
		host := hosts[compID]
		for _, n := range sub.GetNodes() {
			if !hasPrefix(n.GetId(), compID+".") {
				continue
			}
			var cmd rtagent.Command
			require.NoError(t, n.GetTaskState().GetInput().UnmarshalTo(&cmd))
			t.Logf("[%s] exec %s", compID, n.GetId())
			host.execOpResolved(t, ctx, n, cmd.GetOperation(), resolver)
		}
	}

	// Patroni must elect a leader. Probe the REST API via an in-container `timeout`
	// (docker exec ignores context cancellation, so the probe itself must be bounded).
	leader := false
	deadline := time.Now().Add(180 * time.Second)
	for time.Now().Before(deadline) {
		// patronictl shows the cluster; a "Leader" row means a primary was elected.
		// Wrapped in an in-container timeout (docker exec ignores ctx cancellation).
		code, out := hosts["db1"].c2(ctx, []string{"bash", "-lc", "timeout 8 patronictl -c /etc/patroni/patroni.yml list 2>/dev/null || true"})
		if code == 0 && strings.Contains(strings.ToLower(out), "leader") {
			leader = true
			t.Logf("patroni cluster:\n%s", out)
			break
		}
		time.Sleep(5 * time.Second)
	}
	if !leader {
		_, st := hosts["db1"].c2(ctx, []string{"bash", "-lc",
			"echo '== is-active =='; timeout 4 systemctl is-active stroppy-patroni; " +
				"echo '== journal =='; timeout 6 journalctl -u stroppy-patroni --no-pager 2>&1 | tail -40"})
		t.Logf("db1 patroni diagnostics:\n%s", st)
	}
	require.True(t, leader, "patroni cluster never elected a leader")
}

// TestMySQLReplicationFullPipeline brings up a 2-node mysql 8.0 primary/replica
// (GTID) via the recipe across systemd hosts, then asserts a row written on the
// primary replicates to the replica.
func TestMySQLReplicationFullPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = net.Remove(ctx) })

	componentMachine := map[string]string{"db1": "m-db1", "db2": "m-db2"}
	hosts := map[string]*systemdHost{}
	machineIP := map[string]string{}
	for compID, m := range componentMachine {
		h, ip := startSystemdHostOnNetwork(t, ctx, net.Name)
		hosts[compID] = h
		machineIP[m] = ip
	}
	resolved := render.Resolved{}
	for compID, m := range componentMachine {
		resolved[compID] = map[string]string{render.AttrPrivateIP: machineIP[m], render.AttrEndpoint: machineIP[m]}
	}
	resolver := render.NewResolver(resolved)

	comp := func(id string, k domain.Topology_Component_Kind) *domain.Topology_Component {
		return &domain.Topology_Component{Id: id, Kind: k}
	}
	preset := &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_MYSQL, Version: "8.0"},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				{Id: "m-db1", Cores: 2, MemoryGb: 4, Components: []*domain.Topology_Component{comp("db1", domain.Topology_Component_KIND_DATABASE)}},
				{Id: "m-db2", Cores: 2, MemoryGb: 4, Components: []*domain.Topology_Component{comp("db2", domain.Topology_Component_KIND_DATABASE)}},
			},
			Connections: []*domain.Topology_Connection{{From: "db1", To: "db2", Kind: domain.Topology_Connection_KIND_REPLICATION}},
		},
	}
	dag, err := planner.New().Compile(preset, nil)
	require.NoError(t, err)
	sub := findSubDag(t, dag, "install_and_run")

	for _, compID := range []string{"db1", "db2"} {
		host := hosts[compID]
		for _, n := range sub.GetNodes() {
			if !hasPrefix(n.GetId(), compID+".") {
				continue
			}
			var cmd rtagent.Command
			require.NoError(t, n.GetTaskState().GetInput().UnmarshalTo(&cmd))
			t.Logf("[%s] exec %s", compID, n.GetId())
			host.execOpResolved(t, ctx, n, cmd.GetOperation(), resolver)
		}
	}

	// Write on the primary, expect it on the replica.
	code, out := hosts["db1"].sh(t, ctx, `mysql -e "CREATE DATABASE repltest; CREATE TABLE repltest.t(id INT PRIMARY KEY); INSERT INTO repltest.t VALUES (42);"`)
	require.Equalf(t, 0, code, "primary write failed: %s", out)

	replicated := false
	for range 24 {
		code, out = hosts["db2"].c2(ctx, []string{"bash", "-lc", `timeout 8 mysql -N -e "SELECT id FROM repltest.t" 2>/dev/null || true`})
		if code == 0 && strings.Contains(out, "42") {
			replicated = true
			break
		}
		time.Sleep(5 * time.Second)
	}
	if !replicated {
		_, st := hosts["db2"].c2(ctx, []string{"bash", "-lc", `timeout 8 mysql -e "SHOW REPLICA STATUS\G" 2>&1 | grep -iE 'Running|Last_.*Error' | head`})
		t.Logf("db2 replica status:\n%s", st)
	}
	require.True(t, replicated, "row never replicated to the replica")
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

// c2 runs a command (demuxed) returning code + output without failing the test.
// A timeout guards against a hung exec (e.g. patronictl blocking on a dead REST API).
func (h *systemdHost) c2(ctx context.Context, cmd []string) (int, string) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	code, reader, err := h.c.Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		return -1, err.Error()
	}
	return code, readAll(reader)
}
