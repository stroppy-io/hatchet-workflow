//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

// execRaw runs a command with no timeout (apt installs + binary downloads exceed
// the c2 cap), returning code/output/err without failing the test — safe to call
// from a worker goroutine (require/FailNow is not).
func (h *systemdHost) execRaw(ctx context.Context, script string) (int, string, error) {
	code, reader, err := h.c.Exec(ctx, []string{"bash", "-lc", script}, tcexec.Multiplexed())
	if err != nil {
		return -1, "", err
	}
	return code, readAll(reader), nil
}

// execOpResolvedErr is the goroutine-safe variant of execOpResolved: resolves the
// op's bindings and runs it, returning an error instead of calling require.
func (h *systemdHost) execOpResolvedErr(ctx context.Context, node *primitive.Dag_Node, op *ops.Operation, resolver *render.Resolver) error {
	bindings, err := render.NodeBindings(node)
	if err != nil {
		return err
	}
	switch v := op.GetOperation().(type) {
	case *ops.Operation_RunCmd:
		spec := v.RunCmd
		script := spec.GetScript().GetText()
		if script == "" {
			script = joinArgs(spec.GetArgv().GetArgs())
		}
		resolved, rerr := resolver.ResolveText(script, bindings)
		if rerr != nil {
			return rerr
		}
		code, out, eerr := h.execRaw(ctx, resolved)
		if eerr != nil {
			return eerr
		}
		if code != 0 {
			return fmt.Errorf("command failed: %s\n%s", resolved, out)
		}
	case *ops.Operation_WriteFile:
		f := v.WriteFile
		resolved, rerr := resolver.ResolveText(f.GetContent().GetText(), bindings)
		if rerr != nil {
			return rerr
		}
		path := f.GetInfo().GetPath()
		code, out, eerr := h.execRaw(ctx, "mkdir -p \"$(dirname '"+path+"')\" && cat > '"+path+"' <<'STROPPY_EOF'\n"+resolved+"\nSTROPPY_EOF")
		if eerr != nil {
			return eerr
		}
		if code != 0 {
			return fmt.Errorf("write %s failed: %s", path, out)
		}
	default:
		return fmt.Errorf("unsupported op %T", v)
	}
	return nil
}

// runComponentsParallel runs each rank group's components concurrently (independent
// machines; the real runtime leases each agent's commands in parallel), groups in
// order. Within a component the nodes stay sequential (intra-host ordering).
func runComponentsParallel(t *testing.T, ctx context.Context, sub *primitive.Dag, hosts map[string]*systemdHost, resolver *render.Resolver, groups [][]string) {
	t.Helper()
	for _, group := range groups {
		var wg sync.WaitGroup
		errs := make([]error, len(group))
		for i, compID := range group {
			wg.Add(1)
			go func(i int, compID string) {
				defer wg.Done()
				host := hosts[compID]
				for _, n := range sub.GetNodes() {
					if !hasPrefix(n.GetId(), compID+".") {
						continue
					}
					var cmd rtagent.Command
					if uerr := n.GetTaskState().GetInput().UnmarshalTo(&cmd); uerr != nil {
						errs[i] = uerr
						return
					}
					t.Logf("[%s] exec %s", compID, n.GetId())
					if eerr := host.execOpResolvedErr(ctx, n, cmd.GetOperation(), resolver); eerr != nil {
						errs[i] = fmt.Errorf("[%s] %w", compID, eerr)
						return
					}
				}
			}(i, compID)
		}
		wg.Wait()
		for _, e := range errs {
			require.NoError(t, e)
		}
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

	// Rank order: etcd (coordinator) first, then both patroni databases concurrently
	// (they race for the leader lock — exactly what the real runtime does).
	runComponentsParallel(t, ctx, sub, hosts, resolver, [][]string{{"etcd1"}, {"db1", "db2"}})

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

// TestMySQLReplicationFullPipeline (8.0, Ubuntu universe) and
// TestMySQL84ReplicationFullPipeline (8.4 LTS, mysql.com repo) bring up a 2-node
// primary/replica (GTID) via the recipe across systemd hosts, then assert a row
// written on the primary replicates to the replica.
func TestMySQLReplicationFullPipeline(t *testing.T) {
	t.Parallel()
	mysqlReplicationPipeline(t, "8.0")
}

func TestMySQL84ReplicationFullPipeline(t *testing.T) {
	t.Parallel()
	mysqlReplicationPipeline(t, "8.4")
}

func mysqlReplicationPipeline(t *testing.T, version string) {
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
		Database: &domain.Database{Kind: domain.Database_KIND_MYSQL, Version: version},
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

	// Ordered: the primary (db1) must provision the repl user before the replica
	// (db2) runs CHANGE REPLICATION SOURCE — two sequential single-component groups.
	runComponentsParallel(t, ctx, sub, hosts, resolver, [][]string{{"db1"}, {"db2"}})

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

// TestPicodataClusterFullPipeline brings up a 2-instance picodata cluster via the
// recipe across systemd hosts (peers from the rendered picodata.yaml), then asserts
// both instances are up and the pg-wire port accepts connections.
func TestPicodataClusterFullPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = net.Remove(ctx) })

	componentMachine := map[string]string{"pd1": "m-pd1", "pd2": "m-pd2"}
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
		Database: &domain.Database{Kind: domain.Database_KIND_PICODATA, Version: "25.3"},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				{Id: "m-pd1", Cores: 2, MemoryGb: 4, Components: []*domain.Topology_Component{comp("pd1", domain.Topology_Component_KIND_DATABASE)}},
				{Id: "m-pd2", Cores: 2, MemoryGb: 4, Components: []*domain.Topology_Component{comp("pd2", domain.Topology_Component_KIND_DATABASE)}},
			},
			Connections: []*domain.Topology_Connection{{From: "pd1", To: "pd2", Kind: domain.Topology_Connection_KIND_COORDINATION}},
		},
	}
	dag, err := planner.New().Compile(preset, nil)
	require.NoError(t, err)
	sub := findSubDag(t, dag, "install_and_run")

	// Both picodata instances are independent (same kind-rank, peers in the yaml) —
	// bring them up concurrently.
	runComponentsParallel(t, ctx, sub, hosts, resolver, [][]string{{"pd1", "pd2"}})

	// pg-wire (5432) must accept a connection on pd1 — the cluster bootstrapped.
	up := false
	for range 24 {
		code, _ := hosts["pd1"].c2(ctx, []string{"bash", "-lc", "timeout 4 bash -c '</dev/tcp/localhost/5432' 2>/dev/null && echo ok"})
		if code == 0 {
			up = true
			break
		}
		time.Sleep(5 * time.Second)
	}
	if !up {
		_, st := hosts["pd1"].c2(ctx, []string{"bash", "-lc", "echo '== is-active =='; timeout 4 systemctl is-active stroppy-picodata; echo '== journal =='; timeout 6 journalctl -u stroppy-picodata --no-pager 2>&1 | tail -30"})
		t.Logf("pd1 picodata diagnostics:\n%s", st)
	}
	require.True(t, up, "picodata pg-wire never came up")
}

// TestYDBClusterFullPipeline brings up a 3-node ydb storage cluster (mirror-3) via
// the recipe across systemd hosts (each node's static config lists all peers,
// resolved to real IPs; --node selects its id), then asserts every node's storage
// grpc endpoint serves.
func TestYDBClusterFullPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = net.Remove(ctx) })

	ids := []string{"ydb1", "ydb2", "ydb3"}
	hosts := map[string]*systemdHost{}
	machineIP := map[string]string{}
	for _, id := range ids {
		h, ip := startSystemdHostOnNetwork(t, ctx, net.Name)
		hosts[id] = h
		machineIP["m-"+id] = ip
	}
	resolved := render.Resolved{}
	var machines []*domain.Topology_Machine
	var conns []*domain.Topology_Connection
	for i, id := range ids {
		resolved[id] = map[string]string{render.AttrPrivateIP: machineIP["m-"+id], render.AttrEndpoint: machineIP["m-"+id]}
		machines = append(machines, &domain.Topology_Machine{
			Id: "m-" + id, Cores: 2, MemoryGb: 4,
			Components: []*domain.Topology_Component{{Id: id, Kind: domain.Topology_Component_KIND_DATABASE}},
		})
		if i > 0 {
			conns = append(conns, &domain.Topology_Connection{From: id, To: ids[0], Kind: domain.Topology_Connection_KIND_COORDINATION})
		}
	}
	resolver := render.NewResolver(resolved)

	preset := &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_YDB, Version: "24.2"},
		Topology: &domain.Topology{Machines: machines, Connections: conns},
	}
	dag, err := planner.New().Compile(preset, nil)
	require.NoError(t, err)
	sub := findSubDag(t, dag, "install_and_run")

	// All 3 storage nodes are independent machines (same kind-rank) — bring them up
	// concurrently, as the real runtime leases each agent's commands in parallel.
	runComponentsParallel(t, ctx, sub, hosts, resolver, [][]string{ids})

	// Every node's storage grpc endpoint must serve.
	for _, id := range ids {
		up := false
		for range 30 {
			code, _ := hosts[id].c2(ctx, []string{"bash", "-lc", "timeout 4 bash -c '</dev/tcp/localhost/2135' 2>/dev/null && echo ok"})
			if code == 0 {
				up = true
				break
			}
			time.Sleep(3 * time.Second)
		}
		if !up {
			_, j := hosts[id].c2(ctx, []string{"bash", "-lc", "timeout 8 journalctl -u stroppy-ydb-storage --no-pager 2>&1 | grep -iE 'verify|fail|panic|error|invalid|require|expected|domain|location' | grep -viaE '0x' | tail -25"})
			t.Logf("%s ydb diagnostics:\n%s", id, j)
		}
		require.Truef(t, up, "%s storage grpc (2135) never came up", id)
	}
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
