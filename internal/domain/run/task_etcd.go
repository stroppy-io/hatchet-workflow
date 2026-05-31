package run

import (
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// etcdInstallTask downloads the etcd binaries onto the DB nodes (etcd is
// colocated with postgres for HA). The agent only runs the opaque curl|tar
// script the server composes here.
type etcdInstallTask struct {
	client agent.Client
	state  *State
}

func (t *etcdInstallTask) Execute(nc *dag.NodeContext) error {
	// etcd is colocated on DB nodes (first 3).
	targets := t.state.DBTargets()
	if len(targets) > 3 {
		targets = targets[:3]
	}
	// Use platform-wide default etcd version (matches the old agent fallback
	// types.DefaultMonitoring().EtcdVersion).
	version := types.DefaultMonitoring().EtcdVersion
	nc.Log().Info("installing etcd on DB nodes")

	// Download etcd from GitHub releases.
	dlURL := fmt.Sprintf(
		"https://github.com/etcd-io/etcd/releases/download/v%s/etcd-v%s-linux-amd64.tar.gz",
		version, version,
	)
	script := fmt.Sprintf(
		`curl -fsSL --connect-timeout 20 --max-time 120 --retry 3 --retry-delay 5 --retry-connrefused --retry-max-time 300 "%s" -o /tmp/etcd.tar.gz && `+
			`tar xzf /tmp/etcd.tar.gz -C /tmp && `+
			`cp /tmp/etcd-v%s-linux-amd64/etcd /usr/local/bin/etcd && `+
			`cp /tmp/etcd-v%s-linux-amd64/etcdctl /usr/local/bin/etcdctl && `+
			`chmod +x /usr/local/bin/etcd /usr/local/bin/etcdctl && `+
			`rm -rf /tmp/etcd*`,
		dlURL, version, version,
	)

	return sendSeqAll(nc, t.client, targets,
		runCmd("install_etcd", script))
}

// etcdConfigTask renders the etcd env file and starts the etcd cluster on the
// DB nodes. All node naming / cluster-membership computation happens here; the
// agent just writes the env file and runs the start/health scripts.
type etcdConfigTask struct {
	client agent.Client
	state  *State
}

func (t *etcdConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	if len(targets) > 3 {
		targets = targets[:3]
	}
	nc.Log().Info("configuring etcd cluster", zap.Int("nodes", len(targets)))

	// Build initial-cluster string.
	var clusterParts []string
	for i, tgt := range targets {
		name := fmt.Sprintf("etcd%d", i)
		clusterParts = append(clusterParts, fmt.Sprintf("%s=http://%s:2380", name, advertiseHost(tgt)))
	}
	initialCluster := strings.Join(clusterParts, ",")

	dataDir := "/var/lib/etcd"

	// Per-node defaults (match the old agent configEtcd fallbacks).
	const (
		clientURL = "http://0.0.0.0:2379"
		peerURL   = "http://0.0.0.0:2380"
		state     = "new"
	)

	for i, target := range targets {
		name := fmt.Sprintf("etcd%d", i)
		advertiseClient := fmt.Sprintf("http://%s:2379", advertiseHost(target))
		advertisePeer := fmt.Sprintf("http://%s:2380", advertiseHost(target))

		// Write environment file for etcd.
		envContent := fmt.Sprintf(`ETCD_NAME=%s
ETCD_INITIAL_CLUSTER=%s
ETCD_INITIAL_CLUSTER_STATE=%s
ETCD_LISTEN_CLIENT_URLS=%s
ETCD_LISTEN_PEER_URLS=%s
ETCD_ADVERTISE_CLIENT_URLS=%s
ETCD_INITIAL_ADVERTISE_PEER_URLS=%s
`, name, initialCluster, state,
			clientURL, peerURL,
			advertiseClient, advertisePeer)

		// Start etcd via systemd-run (stop old unit if retrying).
		startScript := fmt.Sprintf(
			`mkdir -p %s
systemctl stop etcd 2>/dev/null; systemctl reset-failed etcd 2>/dev/null
systemd-run --unit=etcd --remain-after-exit -- /usr/local/bin/etcd `+
				`--name=%s `+
				`--initial-cluster='%s' `+
				`--initial-cluster-state=%s `+
				`--listen-client-urls=%s `+
				`--listen-peer-urls=%s `+
				`--advertise-client-urls=%s `+
				`--initial-advertise-peer-urls=%s `+
				`--data-dir=%s`,
			dataDir,
			name, initialCluster, state,
			clientURL, peerURL,
			advertiseClient, advertisePeer, dataDir)

		// Wait for etcd to be ready.
		healthScript := `for i in $(seq 1 10); do etcdctl endpoint health --endpoints=http://localhost:2379 2>/dev/null && exit 0; sleep 1; done; echo "etcd not ready" >&2; exit 1`

		cmds := []agent.Command{
			writeFile("config_etcd", "/etc/default/etcd", envContent),
			runCmd("config_etcd", startScript),
			runCmd("config_etcd", healthScript),
		}

		if err := sendSeq(nc, t.client, target, cmds...); err != nil {
			return err
		}
	}

	return nil
}
