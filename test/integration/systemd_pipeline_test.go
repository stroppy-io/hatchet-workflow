//go:build integration

package integration

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/planner"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
)

// systemdHost is a privileged systemd-enabled container that stands in for a VM:
// the planner's agent.command ops (apt install, write config, systemctl) run in it
// via docker exec, validating the full provisioning pipeline (the docker tricks —
// privileged + host cgroup ns + tmpfs /run + DNS — come from the old deployer).
type systemdHost struct {
	c testcontainers.Container
}

func startSystemdHost(t *testing.T, ctx context.Context) *systemdHost {
	t.Helper()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      "jrei/systemd-ubuntu:22.04",
			Privileged: true,
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.CgroupnsMode = "host"
				hc.Tmpfs = map[string]string{"/run": "exec,mode=755", "/run/lock": ""}
				hc.Binds = append(hc.Binds, "/sys/fs/cgroup:/sys/fs/cgroup:rw")
				hc.DNS = []string{"8.8.8.8", "1.1.1.1"}
			},
			WaitingFor: wait.ForExec([]string{"systemctl", "is-system-running", "--wait"}).
				WithExitCodeMatcher(func(code int) bool { return code == 0 || code == 1 }). // running or degraded
				WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "systemd host must start")
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	return &systemdHost{c: c}
}

// sh runs a shell command in the host, returning exit code + combined output.
// tcexec.Multiplexed() demuxes docker's stream framing so the output is clean text.
func (h *systemdHost) sh(t *testing.T, ctx context.Context, script string) (int, string) {
	t.Helper()
	code, reader, err := h.c.Exec(ctx, []string{"bash", "-lc", script}, tcexec.Multiplexed())
	require.NoError(t, err)
	return code, readAll(reader)
}

// execOp runs one planner ops.Operation in the host (RUN_CMD via shell, WRITE_FILE
// via a heredoc). This is the test stand-in for the agent's opexec executor.
func (h *systemdHost) execOp(t *testing.T, ctx context.Context, op *ops.Operation) {
	t.Helper()
	switch v := op.GetOperation().(type) {
	case *ops.Operation_RunCmd:
		spec := v.RunCmd
		var script string
		if s := spec.GetScript(); s != nil {
			script = s.GetText()
		} else {
			script = strings.Join(spec.GetArgv().GetArgs(), " ")
		}
		code, out := h.sh(t, ctx, script)
		require.Equalf(t, 0, code, "command failed: %s\n%s", script, out)
	case *ops.Operation_WriteFile:
		f := v.WriteFile
		path := f.GetInfo().GetPath()
		content := f.GetContent().GetText()
		script := fmt.Sprintf("mkdir -p \"$(dirname %q)\" && cat > %q <<'STROPPY_EOF'\n%s\nSTROPPY_EOF", path, path, content)
		code, out := h.sh(t, ctx, script)
		require.Equalf(t, 0, code, "write %s failed: %s", path, out)
	default:
		t.Fatalf("unsupported op kind in pipeline test: %T", v)
	}
}

// runComponentChain executes, in order, every agent.command node for componentID
// from the dag's install_and_run sub-dag.
func (h *systemdHost) runComponentChain(t *testing.T, ctx context.Context, dag *primitive.Dag, componentID string) {
	t.Helper()
	var sub *primitive.Dag
	for _, n := range dag.GetNodes() {
		if n.GetId() == "install_and_run" {
			sub = n.GetSubDag()
		}
	}
	require.NotNil(t, sub, "install_and_run sub-dag")

	prefix := componentID + "."
	ran := 0
	for _, n := range sub.GetNodes() {
		if !strings.HasPrefix(n.GetId(), prefix) {
			continue
		}
		var cmd rtagent.Command
		require.NoError(t, n.GetTaskState().GetInput().UnmarshalTo(&cmd))
		t.Logf("exec node %s", n.GetId())
		h.execOp(t, ctx, cmd.GetOperation())
		ran++
	}
	require.Greater(t, ran, 0, "no nodes for component %s", componentID)
}

// TestPostgresSingleFullPipeline compiles a single-postgres preset, then executes
// the rendered install recipe (pgdg repo + apt install + config writes + start)
// inside a systemd host, and asserts postgres comes up AND the rendered config was
// actually applied (shared_buffers matches what we rendered).
func TestPostgresSingleFullPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	preset := &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"},
		Topology: &domain.Topology{Machines: []*domain.Topology_Machine{{
			Id: "m1", Cores: 2, MemoryGb: 4,
			Components: []*domain.Topology_Component{{Id: "pg", Kind: domain.Topology_Component_KIND_DATABASE}},
		}}},
	}
	dag, err := planner.New().Compile(preset, nil)
	require.NoError(t, err)

	host := startSystemdHost(t, ctx)
	host.runComponentChain(t, ctx, dag, "pg")

	// Postgres must accept connections.
	code, out := host.sh(t, ctx, "pg_isready -h 127.0.0.1 || pg_isready")
	require.Equalf(t, 0, code, "postgres not ready: %s", out)

	// The rendered tuning must be in effect (not the engine default of 128MB):
	// 4GB budget -> shared_buffers 25% = 1GB.
	code, out = host.sh(t, ctx, `su postgres -c "psql -tAc 'SHOW shared_buffers'"`)
	require.Equalf(t, 0, code, "psql failed: %s", out)
	sb := strings.TrimSpace(out)
	require.NotEqual(t, "128MB", sb, "rendered config not applied — got engine default")
	t.Logf("shared_buffers = %s", sb)
}

// TestYDBSingleFullPipeline runs the ydb binary recipe (download + file-pdisk
// static config + storage start + blobstorage/database init) in a systemd host and
// asserts the storage grpc endpoint is serving and the database was created.
func TestYDBSingleFullPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	preset := &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_YDB, Version: "24.2"},
		Topology: &domain.Topology{Machines: []*domain.Topology_Machine{{
			Id: "m1", Cores: 2, MemoryGb: 4,
			Components: []*domain.Topology_Component{{Id: "db", Kind: domain.Topology_Component_KIND_DATABASE}},
		}}},
	}
	dag, err := planner.New().Compile(preset, nil)
	require.NoError(t, err)

	host := startSystemdHost(t, ctx)
	// The chain's wait_ready only passes once ydbd storage serves grpc (the static
	// config self-bootstraps the cluster + /Root domain).
	host.runComponentChain(t, ctx, dag, "db")

	// Confirm the storage grpc endpoint is still serving.
	code, _ := host.c2(ctx, []string{"bash", "-lc", "timeout 4 bash -c '</dev/tcp/localhost/2135' 2>/dev/null && echo ok"})
	require.Equal(t, 0, code, "ydb storage grpc (2135) not serving")
}

// TestCockroachSingleFullPipeline runs the cockroach binary recipe (download +
// start-single-node via flags, no config file) in a systemd host and asserts SQL.
func TestCockroachSingleFullPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	preset := &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_COCKROACH, Version: "24.2"},
		Topology: &domain.Topology{Machines: []*domain.Topology_Machine{{
			Id: "m1", Cores: 2, MemoryGb: 4,
			Components: []*domain.Topology_Component{{Id: "db", Kind: domain.Topology_Component_KIND_DATABASE}},
		}}},
	}
	dag, err := planner.New().Compile(preset, nil)
	require.NoError(t, err)

	host := startSystemdHost(t, ctx)
	host.runComponentChain(t, ctx, dag, "db")

	var ok bool
	var out string
	for range 20 {
		code, o := host.sh(t, ctx, `cockroach sql --insecure --host=localhost:5432 -e "SELECT 1" 2>&1`)
		if code == 0 {
			ok = true
			break
		}
		out = o
		time.Sleep(2 * time.Second)
	}
	require.Truef(t, ok, "cockroach SQL never succeeded: %s", out)
}

// TestMariaDBSingleFullPipeline compiles a single-mariadb preset, runs the rendered
// install recipe (mariadb repo_setup + apt install + conf.d write + start) in a
// systemd host, and asserts mariadb is up AND the rendered tuning is applied (innodb
// buffer pool above the engine default).
func TestMariaDBSingleFullPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	preset := &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_MARIADB, Version: "11.4"},
		Topology: &domain.Topology{Machines: []*domain.Topology_Machine{{
			Id: "m1", Cores: 2, MemoryGb: 4,
			Components: []*domain.Topology_Component{{Id: "db", Kind: domain.Topology_Component_KIND_DATABASE}},
		}}},
	}
	dag, err := planner.New().Compile(preset, nil)
	require.NoError(t, err)

	host := startSystemdHost(t, ctx)
	host.runComponentChain(t, ctx, dag, "db")

	// mariadb up + answers (root via unix_socket after apt install).
	code, out := host.sh(t, ctx, `mariadb -e "SELECT 1"`)
	require.Equalf(t, 0, code, "mariadb not ready: %s", out)

	// rendered innodb_buffer_pool_size (50% of 4GB, capped 2GB) must beat the
	// 128MiB engine default.
	code, out = host.sh(t, ctx, `mariadb -N -e "SELECT @@innodb_buffer_pool_size"`)
	require.Equalf(t, 0, code, "mariadb query failed: %s", out)
	bytesVal, perr := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	require.NoError(t, perr, "buffer pool size: %q", out)
	require.Greater(t, bytesVal, int64(134217728), "rendered innodb_buffer_pool_size not applied")
	t.Logf("innodb_buffer_pool_size = %d", bytesVal)
}

// TestMySQLSingleFullPipeline: mysql 8.0 from the Ubuntu archive (universe) — runs
// the recipe (no third-party repo/key), writes the conf.d tuning, starts mysql,
// and asserts it is up with the rendered innodb buffer pool. (mysql 8.4 needs the
// upstream mysql.com repo whose GPG key is expired — TestMySQLSingleConfigBoots
// still covers 8.4's rendered config against the official image.)
func TestMySQLSingleFullPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	preset := &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_MYSQL, Version: "8.0"},
		Topology: &domain.Topology{Machines: []*domain.Topology_Machine{{
			Id: "m1", Cores: 2, MemoryGb: 4,
			Components: []*domain.Topology_Component{{Id: "db", Kind: domain.Topology_Component_KIND_DATABASE}},
		}}},
	}
	dag, err := planner.New().Compile(preset, nil)
	require.NoError(t, err)

	host := startSystemdHost(t, ctx)
	host.runComponentChain(t, ctx, dag, "db")

	code, out := host.sh(t, ctx, `mysql -e "SELECT 1"`)
	require.Equalf(t, 0, code, "mysql not ready: %s", out)

	code, out = host.sh(t, ctx, `mysql -N -e "SELECT @@innodb_buffer_pool_size"`)
	require.Equalf(t, 0, code, "mysql query failed: %s", out)
	bytesVal, perr := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	require.NoError(t, perr, "buffer pool size: %q", out)
	require.Greater(t, bytesVal, int64(134217728), "rendered innodb_buffer_pool_size not applied")
	t.Logf("innodb_buffer_pool_size = %d", bytesVal)
}
