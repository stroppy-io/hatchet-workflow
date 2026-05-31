package run

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbconfig"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// picoInstallTask installs the picodata package on every DB node. The agent
// only runs the opaque apt script the server composes here.
type picoInstallTask struct {
	client   CommandSink
	state    *State
	version  string
	topology *types.PicodataTopology
	pkg      *types.Package
}

func (t *picoInstallTask) Execute(nc *NodeContext) error {
	targets := t.state.DBTargets()
	if t.pkg == nil {
		return fmt.Errorf("install picodata: no package provided")
	}
	nc.Log().Info("installing picodata on targets")
	return sendSeqAll(nc, t.client, targets,
		aptCmd("install_picodata", installPackageScript(*t.pkg)))
}

// picoConfigTask renders picodata.yaml per instance and starts the cluster via
// systemd-run. All rendering happens here; the agent just writes the config
// file and runs the start/readiness scripts.
type picoConfigTask struct {
	client    CommandSink
	state     *State
	topology  *types.PicodataTopology
	overrides map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "picodata.yaml"
}

func (t *picoConfigTask) Execute(nc *NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring picodata cluster")

	// Peer list: advertise host of every instance. Container name on Docker,
	// internal IP on YC VMs.
	peers := make([]string, len(targets))
	for i, tgt := range targets {
		peers[i] = advertiseHost(tgt)
	}

	// First peer is the bootstrap instance; all others join via it.
	firstPeer := peers[0]
	if !strings.Contains(firstPeer, ":") {
		firstPeer = firstPeer + ":3301"
	}

	spec := types.MachineSpec{}
	if len(t.topology.Instances) > 0 {
		spec = t.topology.Instances[0]
	}
	// Server-side memory budget for "25%"-style defaults. If the spec memory is
	// 0, pass it through as-is (renderers handle 0).
	memMB := spec.MemoryMB

	// Re-derive memtx bytes from defaults so systemd-run can pass it as env.
	// (User may have edited the body to a different value, but the env var
	// historically came from the same defaults map; keep that behavior so an
	// override of memtx in the YAML doesn't silently disagree with the env.)
	defaults := types.PicodataDefaults("25.3")
	dbconfig.ResolveMemoryPercents(defaults, memMB)
	for k, v := range t.topology.InstanceOptions {
		defaults[k] = v
	}
	memtxBytes := picodataMemtxBytesFromDefault(defaults["memtx_memory"])

	dataDir := "/var/lib/picodata"
	confPath := "/etc/picodata/picodata.yaml"

	for i, target := range targets {
		// Use advertise host so other nodes can resolve this instance.
		hostname := advertiseHost(target)

		// Config body: user override wins, else render server-side.
		body := t.overrides["picodata.yaml"]
		if body == "" {
			body = dbconfig.RenderPicodataConf(dbconfig.RenderPicodataConfOpts{
				Replication:   t.topology.Replication,
				Options:       t.topology.InstanceOptions,
				TotalMemoryMB: memMB,
			})
		}
		body = dbconfig.SubstitutePicodataPlaceholders(body, hostname)

		instanceName := fmt.Sprintf("instance-%d", i)

		// Start picodata via systemd-run (stop old unit if retrying).
		startScript := fmt.Sprintf(
			`systemd-run --unit=picodata \
  --setenv=PICODATA_ADMIN_PASSWORD=T0psecret \
  --setenv=PICODATA_MEMTX_MEMORY=%d \
  picodata run --config %s --peer %s --instance-name %s --instance-dir %s`,
			memtxBytes, confPath, firstPeer, instanceName, dataDir)

		cmds := []agent.Command{
			runCmd("config_picodata", "mkdir -p /etc/picodata"),
			writeFile("config_picodata", confPath, body),
			runCmd("config_picodata", fmt.Sprintf("mkdir -p %s", dataDir)),
			runCmd("config_picodata", "systemctl stop picodata 2>/dev/null; systemctl reset-failed picodata 2>/dev/null"),
			runCmd("config_picodata", startScript),
			// Wait for picodata readiness.
			runCmd("config_picodata", `for i in $(seq 1 10); do curl -sf http://localhost:8081/api/v1/health/ready 2>/dev/null && exit 0; sleep 1; done; echo "picodata not ready after 10s:" >&2; cat /var/lib/picodata/picodata.log >&2; exit 1`),
		}

		if err := sendSeq(nc, t.client, target, cmds...); err != nil {
			return err
		}
	}

	// Store effective config.
	ec := map[string]string{
		"kind":      "picodata",
		"instances": fmt.Sprintf("%d", len(targets)),
		"shards":    fmt.Sprintf("%d", t.topology.Shards),
		"rf":        fmt.Sprintf("%d", t.topology.Replication),
	}
	if len(t.topology.Instances) > 0 {
		inst := t.topology.Instances[0]
		ec["per_node"] = fmt.Sprintf("%d vCPU / %d MB / %d GB", inst.CPUs, inst.MemoryMB, inst.DiskGB)
	}
	for k, v := range t.topology.InstanceOptions {
		ec[k] = v
	}
	t.state.SetEffectiveConfig("database", ec)

	return nil
}

// picodataMemtxBytesFromDefault converts an "<N>MB" string from the picodata
// defaults map into bytes for the PICODATA_MEMTX_MEMORY env var. Falls back
// to 2 GiB when missing or unparseable — the on-host default of 64 MB is too
// small for TPC-C seed data.
func picodataMemtxBytesFromDefault(s string) int {
	v := strings.TrimSuffix(s, "MB")
	var mb int
	if _, err := fmt.Sscanf(v, "%d", &mb); err == nil && mb > 0 {
		return mb * 1024 * 1024
	}
	return 2 * 1024 * 1024 * 1024
}
