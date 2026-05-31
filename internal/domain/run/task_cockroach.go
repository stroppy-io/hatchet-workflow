package run

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// CockroachDB is a homogeneous-node distributed SQL DB:
//   - install: pull tarball from binaries.cockroachdb.com on every node
//   - configure: start each node with --advertise-addr and --join pointing
//                at all other nodes
//   - init: run `cockroach init` once on the first node, then apply any
//           SET CLUSTER SETTING from topology.Options
//
// Three small tasks, mirroring the YDB shape (install / configure / init)
// but without the storage-vs-compute role split — every node is the same.
// The agent only runs the opaque primitives the server composes here.

// resolveCockroachVersion maps a short version like "24.2" to the full patch
// release available at binaries.cockroachdb.com. Updated as new releases ship.
var cockroachVersionMap = map[string]string{
	"24.2": "24.2.4",
	"24.1": "24.1.5",
	"23.2": "23.2.10",
}

func resolveCockroachVersion(v string) string {
	if v == "" {
		return "24.2.4"
	}
	if full, ok := cockroachVersionMap[v]; ok {
		return full
	}
	return v
}

// cockroachInstallTask pulls the cockroach tarball on every DB node and sets
// up the dedicated cockroach user/dirs.
type cockroachInstallTask struct {
	client  agent.Client
	state   *State
	version string
}

func (t *cockroachInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing cockroach on targets")

	version := resolveCockroachVersion(t.version)
	url := fmt.Sprintf("https://binaries.cockroachdb.com/cockroach-v%s.linux-amd64.tgz", version)

	// The old agent did exec.LookPath("/opt/cockroach/cockroach") to skip the
	// download. On the dumb agent there is no Go-side LookPath, so guard with a
	// bash `test -x`: only download when the binary is missing. curl|tar is not
	// an apt operation, so it doesn't strictly need the exclusive lock.
	downloadScript := fmt.Sprintf(
		`test -x /opt/cockroach/cockroach && echo "cockroach already installed, skipping download" || `+
			`(mkdir -p /opt/cockroach && curl -fSL %s | tar -xz --strip-components=1 -C /opt/cockroach && `+
			`ln -sf /opt/cockroach/cockroach /usr/local/bin/cockroach)`, url)

	cmds := []agent.Command{
		runCmd("install_cockroach", downloadScript),
		// Dedicated cockroach user — daemon shouldn't run as root.
		runCmd("install_cockroach", "groupadd -f cockroach && (id -u cockroach &>/dev/null || useradd cockroach -g cockroach -d /var/lib/cockroach -m)"),
		runCmd("install_cockroach", "mkdir -p /var/lib/cockroach && chown -R cockroach:cockroach /var/lib/cockroach"),
	}
	return sendSeqAll(nc, t.client, targets, cmds...)
}

// cockroachConfigTask starts every node in the cluster with --advertise-addr
// and --join pointing at the other nodes. All nodes must come up concurrently
// to discover each other via --join; init runs after.
type cockroachConfigTask struct {
	client   agent.Client
	state    *State
	topology *types.CockroachTopology
}

func (t *cockroachConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring cockroach cluster")

	// Build the --join peer list. Use advertiseHost (internal host with a
	// fallback to Host) so cluster traffic stays in-VPC; cockroach addresses
	// peers by the same string we put in --advertise-addr, so they must match.
	peers := make([]string, len(targets))
	for i, tgt := range targets {
		peers[i] = advertiseHost(tgt)
	}

	memMB := t.topology.Nodes.MemoryMB
	cacheMB := 0
	if memMB > 0 {
		cacheMB = memMB / 4 // 25% of RAM, matching CockroachLabs' standard recommendation
	}
	sqlMB := 0
	if memMB > 0 {
		sqlMB = memMB / 4
	}

	for i, target := range targets {
		advHost := advertiseHost(target)

		join := ""
		if len(peers) > 0 {
			// Strip our own host so cockroach doesn't list itself in --join
			// (which it tolerates but log-spams about).
			p := make([]string, 0, len(peers))
			for _, peer := range peers {
				if peer == advHost {
					continue
				}
				p = append(p, peer)
			}
			if len(p) > 0 {
				join = fmt.Sprintf("--join=%s ", strings.Join(p, ","))
			}
		}

		startScript := fmt.Sprintf(
			`systemctl stop cockroach 2>/dev/null; systemctl reset-failed cockroach 2>/dev/null
echo "starting cockroach node #%d (cache=%dMB, sql-memory=%dMB)..."
systemd-run --unit=cockroach --uid=cockroach --gid=cockroach `+
				`/usr/local/bin/cockroach start --insecure `+
				`--advertise-addr=%s:26257 `+
				`--listen-addr=0.0.0.0:26257 `+
				`--http-addr=0.0.0.0:8080 `+
				`--store=/var/lib/cockroach `+
				`--cache=%dMiB --max-sql-memory=%dMiB `+
				`%s`+
				`--background
# Wait for the SQL port to accept connections.
for i in $(seq 1 30); do (echo > /dev/tcp/localhost/26257) 2>/dev/null && { echo "cockroach node ready"; exit 0; }; sleep 1; done
journalctl -u cockroach --no-pager -n 50 2>/dev/null || true
exit 1`,
			i, cacheMB, sqlMB, advHost, cacheMB, sqlMB, join)

		// Start each node concurrently — nodes have to be up at the same time
		// to discover each other via --join.
		if err := t.client.Send(nc, target, runCmd("config_cockroach", startScript)); err != nil {
			return err
		}
	}

	// Store effective config.
	n := t.topology.Nodes
	ec := map[string]string{
		"kind":  "cockroach",
		"nodes": fmt.Sprintf("%d× %d vCPU / %d MB / %d GB", n.Count, n.CPUs, n.MemoryMB, n.DiskGB),
	}
	for k, v := range t.topology.Options {
		ec[k] = v
	}
	t.state.SetEffectiveConfig("database", ec)

	// DB endpoint is set in task_infra (port 26257 from the protocol
	// registry); nothing else to do here.
	return nil
}

// cockroachInitTask runs the one-shot `cockroach init` on the first node to
// turn the running nodes into a working cluster, then applies any
// SET CLUSTER SETTING from topology.Options.
type cockroachInitTask struct {
	client   agent.Client
	state    *State
	topology *types.CockroachTopology
}

func (t *cockroachInitTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	if len(targets) == 0 {
		return nil
	}
	first := targets[0]
	host := advertiseHost(first)
	port := 26257
	nc.Log().Info("initialising cockroach cluster")

	cmds := make([]agent.Command, 0, 1+len(t.topology.Options))
	// `cockroach init` is idempotent — reports "cluster has already been
	// initialised" on rerun.
	cmds = append(cmds, runCmd("init_cockroach", fmt.Sprintf(
		`echo "initialising cockroach cluster via %s:%d..."
/usr/local/bin/cockroach init --insecure --host=%s:%d 2>&1 | tee /tmp/crdb-init.log; grep -q "already been initialized" /tmp/crdb-init.log && exit 0 || exit ${PIPESTATUS[0]}`,
		host, port, host, port)))

	for k, v := range t.topology.Options {
		stmt := fmt.Sprintf("SET CLUSTER SETTING %s = '%s';", k, v)
		cmds = append(cmds, runCmd("init_cockroach", fmt.Sprintf(
			`/usr/local/bin/cockroach sql --insecure --host=%s:%d --execute="%s"`,
			host, port, stmt)))
	}

	return sendSeq(nc, t.client, first, cmds...)
}
