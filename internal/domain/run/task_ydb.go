package run

import (
	"fmt"
	"strings"
	"sync"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbconfig"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// ydbVersionMap maps short version labels (from UI) to full patch versions
// available at binaries.ydb.tech/release/. Update when new YDB releases ship.
var ydbVersionMap = map[string]string{
	"25.2": "25.2.1.24",
	"25.1": "25.1.4.7",
	"24.4": "24.4.4.12",
	"24.3": "24.3.15.5",
	"24.2": "24.2.7",
	"24.1": "24.1.18",
}

// resolveYDBVersion maps a short version like "25.2" to the full "25.2.1.24".
// If the input is already a full version (e.g. "25.2.1.24"), returns it as-is.
func resolveYDBVersion(v string) string {
	if v == "" {
		return "25.2.1.24"
	}
	if full, ok := ydbVersionMap[v]; ok {
		return full
	}
	return v // assume full version was passed directly
}

type ydbInstallTask struct {
	client   CommandSink
	state    *State
	version  string
	topology *types.YDBTopology
	pkg      *types.Package
}

func (t *ydbInstallTask) Execute(nc *NodeContext) error {
	// Install ydbd binaries on every YDB node (storage + compute alike).
	targets := t.state.DBTargets()
	nc.Log().Info("installing YDB on targets")

	// Map short versions (e.g. "25.2") to full patch versions available at binaries.ydb.tech.
	ydbVersion := resolveYDBVersion(t.version)
	downloadURL := fmt.Sprintf("https://binaries.ydb.tech/release/%s/ydbd-%s-linux-amd64.tar.gz", ydbVersion, ydbVersion)

	// Idempotency guard: skip the download when ydbd is already on disk.
	// YDB CLI not needed — ydbd admin commands are used directly. Skip to save time.
	script := fmt.Sprintf(`set -e
if [ -x /opt/ydb/bin/ydbd ]; then
  echo "ydbd already installed, skipping download"
else
  echo "downloading ydbd %s from %s..."
  mkdir -p /opt/ydb && curl -fSL %s | tar -xz --strip-component=1 -C /opt/ydb
fi
groupadd -f ydb && (id -u ydb &>/dev/null || useradd ydb -g ydb)
mkdir -p /opt/ydb/cfg /ydb_data && chown -R ydb:ydb /ydb_data`, ydbVersion, downloadURL, downloadURL)

	return sendSeqAll(nc, t.client, targets, runCmd("install_ydb", script))
}

type ydbConfigTask struct {
	client     CommandSink
	state      *State
	topology   *types.YDBTopology
	overrides  map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "ydb.yaml:storage"
	pgwirePort int               // > 0 → ydbd starts with --pgwire-port (set when run uses ydb-pgwire protocol)
}

func (t *ydbConfigTask) Execute(nc *NodeContext) error {
	// Static (storage) daemon runs on storage nodes only. In combined mode
	// these are also the only YDB nodes, so this matches DBTargets().
	targets := t.state.YDBStorageTargets()
	nc.Log().Info("configuring YDB static nodes")

	hosts := make([]string, len(targets))
	locations := make([]string, len(targets))
	for i, tgt := range targets {
		hosts[i] = advertiseHost(tgt)
		locations[i] = tgt.Zone
	}

	ft := t.topology.FaultTolerance
	if ft == "" {
		ft = "none"
	}

	// When the storage spec attaches secondary disks, point YDB's pdisks at
	// the raw devices. Yandex Cloud surfaces secondary disks at
	// /dev/disk/by-id/virtio-<DeviceName>; one pdisk per attached disk.
	blockDevicePaths := allBlockDevicePaths(t.topology.Storage.SecondaryDisks)

	// Memory budget for the storage daemon. In combined mode (no separate
	// Database spec) the same node also runs ydbd-database, and each daemon
	// claims hard_limit_bytes = memMB × 0.85. With both reading the full
	// machine memory, the combined hard limit can exceed physical RAM and
	// Linux OOM-kills one of them mid-run — which surfaces to clients as
	// BAD_SESSION storms. Halve the budget so both fit.
	storageMemMB := t.topology.Storage.MemoryMB
	combined := t.topology.Database == nil
	if combined && storageMemMB > 0 {
		storageMemMB /= 2
	}

	diskPath := "/ydb_data"
	confPath := "/opt/ydb/cfg/config.yaml"

	// Disk size: use allocated disk minus 2 GB headroom for OS/logs, min 10 GB.
	diskGBalloc := t.topology.Storage.DiskGB
	if diskGBalloc <= 0 {
		diskGBalloc = 80
	}
	pdiskGB := diskGBalloc - 2
	if pdiskGB < 10 {
		pdiskGB = 10
	}

	// Render the storage config.yaml body server-side: user override wins,
	// else render from topology. Host placeholders are substituted with the
	// storage host list (same for every node).
	body := t.overrides["ydb.yaml:storage"]
	if body == "" {
		body = dbconfig.RenderYDBStorageConf(dbconfig.RenderYDBConfOpts{
			HostCount:         len(hosts),
			HostLocations:     locations,
			DiskPath:          diskPath,
			BlockDevicePaths:  blockDevicePaths,
			CPUs:              t.topology.Storage.CPUs,
			MemoryMB:          storageMemMB,
			FaultTolerance:    ft,
			FailureDomainType: t.topology.FailureDomainType,
			DefaultDiskType:   t.topology.DefaultDiskType,
		})
	}
	body = dbconfig.SubstituteYDBHostPlaceholders(body, hosts)

	pgwireFlag := ""
	if t.pgwirePort > 0 {
		pgwireFlag = fmt.Sprintf("--pgwire-port %d ", t.pgwirePort)
	}

	// Start all static nodes in parallel — YDB needs all nodes up to form a cluster.
	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for i, target := range targets {
		advHost := advertiseHost(target)

		cmds := []agent.Command{
			writeFile("config_ydb", confPath, body),
		}

		// Prepare pdisks. With raw block devices we point YDB at each device
		// directly — no filesystem, no /ydb_data dir. Without any, fall back to
		// the file-backed pdisk that's used in dev/Docker (single pdisk only).
		var prepScript strings.Builder
		prepScript.WriteString("set -e\n")
		if len(blockDevicePaths) == 0 {
			fmt.Fprintf(&prepScript, "mkdir -p %s && chown -R ydb:ydb %s\n", diskPath, diskPath)
			fmt.Fprintf(&prepScript, "test -f %s/pdisk.data || truncate -s %dG %s/pdisk.data\n", diskPath, pdiskGB, diskPath)
			fmt.Fprintf(&prepScript, "chown ydb:ydb %s/pdisk.data\n", diskPath)
		} else {
			// Raw block devices: ydbd runs as user "ydb" via systemd-run, but
			// /dev/vdX is root:disk by default. chown each device so the daemon
			// can open it. chown follows the symlink in /dev/disk/by-id, so we
			// can target the friendly paths.
			for _, p := range blockDevicePaths {
				fmt.Fprintf(&prepScript, "chown ydb:ydb %s\n", p)
			}
		}
		pdiskTargets := blockDevicePaths
		if len(pdiskTargets) == 0 {
			pdiskTargets = []string{diskPath + "/pdisk.data"}
		}
		fmt.Fprintf(&prepScript, "echo \"preparing %d YDB pdisk(s)...\"\n", len(pdiskTargets))
		for _, p := range pdiskTargets {
			fmt.Fprintf(&prepScript, "LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd admin bs disk obliterate %s\n", p)
		}
		cmds = append(cmds, runCmd("config_ydb", prepScript.String()))

		// Ensure hostname matches hosts[].host so YDB can detect its node ID.
		// On Docker: hostname already set to container name.
		// On YC VMs: hostname is auto-assigned, need to set it to advertiseHost (internal IP).
		if advHost != "" {
			cmds = append(cmds, runCmd("config_ydb", fmt.Sprintf(
				"hostnamectl set-hostname %s 2>/dev/null || hostname %s", advHost, advHost)))
		}

		// Start static node. When the run picked the ydb-pgwire protocol the
		// run task sets PgwirePort > 0 and we add --pgwire-port to expose the
		// experimental postgres-wire surface alongside the gRPC one.
		startScript := fmt.Sprintf(`set -e
systemctl stop ydbd-storage 2>/dev/null; systemctl reset-failed ydbd-storage 2>/dev/null
echo "starting YDB static (storage) node..."
systemd-run --unit=ydbd-storage --uid=ydb --gid=ydb `+
			`--setenv=LD_LIBRARY_PATH=/opt/ydb/lib `+
			`/opt/ydb/bin/ydbd server `+
			`--yaml-config %s `+
			`--grpc-port 2136 --ic-port 19001 --mon-port 8765 `+
			`%s`+
			`--node static`, confPath, pgwireFlag)
		cmds = append(cmds, runCmd("config_ydb", startScript))

		// Readiness loop; on failure dump the journal for debugging. The
		// subshell wraps the loop so a non-success run reaches the journal
		// dump and exits nonzero (the bare loop would otherwise exit 0).
		readyScript := `if (for i in $(seq 1 60); do (echo > /dev/tcp/localhost/2136) 2>/dev/null && exit 0; sleep 1; done; exit 1); then
  echo "YDB static node started"
else
  journalctl -u ydbd-storage --no-pager -n 50 2>/dev/null || true
  echo "ydbd-storage did not start" >&2
  exit 1
fi`
		cmds = append(cmds, runCmd("config_ydb", readyScript))

		wg.Add(1)
		go func(idx int, tgt agent.Target, c []agent.Command) {
			defer wg.Done()
			errs[idx] = sendSeq(nc, t.client, tgt, c...)
		}(i, target, cmds)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	// Store effective config for UI display.
	st := t.topology.Storage
	diskGB := st.DiskGB
	if diskGB <= 0 {
		diskGB = 80
	}
	memMB := st.MemoryMB
	if memMB <= 0 {
		memMB = 4096
	}
	cpus := st.CPUs
	if cpus <= 0 {
		cpus = 2
	}
	pdiskGBeff := diskGB - 2
	if pdiskGBeff < 10 {
		pdiskGBeff = 10
	}
	hardMB := memMB * 85 / 100
	nodes := fmt.Sprintf("%d storage", st.Count)
	if t.topology.Database != nil {
		nodes += fmt.Sprintf(" + %d compute", t.topology.Database.Count)
	} else {
		nodes += " (combined)"
	}
	effective := map[string]string{
		"kind":           "ydb",
		"nodes":          nodes,
		"per_node":       fmt.Sprintf("%d vCPU / %d MB / %d GB", cpus, memMB, diskGB),
		"pdisk_gb":       fmt.Sprintf("%d", pdiskGBeff),
		"mem_limit":      fmt.Sprintf("%d MB", hardMB),
		"cpu_count":      fmt.Sprintf("%d", cpus),
		"erasure":        ft,
		"storage_groups": fmt.Sprintf("%d", ydbStorageGroups(t.topology)),
		"db_path":        t.topology.DatabasePath,
	}
	if t.topology.FailureDomainType != "" {
		effective["failure_domain"] = t.topology.FailureDomainType
	}
	t.state.SetEffectiveConfig("database", effective)

	return nil
}

type ydbInitTask struct {
	client   CommandSink
	state    *State
	topology *types.YDBTopology
}

func (t *ydbInitTask) Execute(nc *NodeContext) error {
	// Cluster init runs once against the static endpoint — first storage node.
	targets := t.state.YDBStorageTargets()
	if len(targets) == 0 {
		return fmt.Errorf("no YDB storage targets for cluster init")
	}

	first := targets[0]
	host := advertiseHost(first)

	dbPath := t.topology.DatabasePath
	if dbPath == "" {
		dbPath = "/Root/testdb"
	}

	endpoint := fmt.Sprintf("grpc://%s:2136", host)
	confPath := "/opt/ydb/cfg/config.yaml"
	storageGroups := ydbStorageGroups(t.topology)
	storagePoolKind := ydbStoragePoolKind(t.topology)

	nc.Log().Info("initializing YDB cluster")

	// Initialize blobstorage (retry — cluster needs time to form quorum).
	blobScript := fmt.Sprintf(`echo "initializing YDB blobstorage..."
for i in $(seq 1 30); do LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd -s %s admin blobstorage config init --yaml-file %s 2>&1 && exit 0; sleep 2; done; exit 1`,
		endpoint, confPath)

	dbScript := fmt.Sprintf(`echo "creating YDB database %s..."
for i in $(seq 1 15); do LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd -s %s admin database %s create %s:%d 2>&1 && { echo "YDB cluster initialized"; exit 0; }; sleep 2; done; exit 1`,
		dbPath, endpoint, dbPath, storagePoolKind, storageGroups)

	return sendSeq(nc, t.client, first,
		runCmd("init_ydb", blobScript),
		runCmd("init_ydb", dbScript),
	)
}

type ydbStartDBTask struct {
	client     CommandSink
	state      *State
	topology   *types.YDBTopology
	overrides  map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "ydb.yaml:database"
	pgwirePort int               // > 0 → ydbd starts with --pgwire-port (set when run uses ydb-pgwire protocol)
}

func (t *ydbStartDBTask) Execute(nc *NodeContext) error {
	// Dynamic (database) daemon: in split mode it runs on dedicated compute
	// nodes; in combined mode the storage nodes themselves host it co-located.
	targets := t.state.YDBDatabaseTargets()
	if len(targets) == 0 {
		targets = t.state.YDBStorageTargets()
	}
	nc.Log().Info("starting YDB database nodes")

	// staticHosts is always the full storage host list — that's what the
	// database daemon uses as its --node-broker list, regardless of which
	// node it's running on.
	storageTargets := t.state.YDBStorageTargets()
	staticHosts := make([]string, len(storageTargets))
	staticLocations := make([]string, len(storageTargets))
	for i, tgt := range storageTargets {
		staticHosts[i] = advertiseHost(tgt)
		staticLocations[i] = tgt.Zone
	}

	dbPath := t.topology.DatabasePath
	if dbPath == "" {
		dbPath = "/Root/testdb"
	}

	blockDevicePaths := allBlockDevicePaths(t.topology.Storage.SecondaryDisks)
	dbConfPath := "/opt/ydb/cfg/database.yaml"

	// --node-broker flags — the full static endpoint list.
	var brokerFlags strings.Builder
	for _, ep := range staticHosts {
		fmt.Fprintf(&brokerFlags, " --node-broker grpc://%s:2136", ep)
	}

	// When the run picked ydb-pgwire, expose the postgres-wire surface on
	// the dynamic node so clients can hit it directly rather than via the
	// static node's gRPC port.
	pgwireFlag := ""
	if t.pgwirePort > 0 {
		pgwireFlag = fmt.Sprintf("--pgwire-port %d ", t.pgwirePort)
	}

	// Start all dynamic nodes in parallel.
	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for i, target := range targets {
		// Use database node specs if split mode, otherwise storage specs.
		// In combined mode (Database == nil) the same node also runs
		// ydbd-storage, and each daemon claims hard_limit_bytes = memMB ×
		// 0.85 independently. Halve the budget here so the two daemons
		// together fit in RAM (the storage side does the same in
		// ydbConfigTask). In split mode each daemon runs on its own node,
		// so the database-spec memory is used as-is.
		memMB := t.topology.Storage.MemoryMB
		cpus := t.topology.Storage.CPUs
		if t.topology.Database != nil {
			memMB = t.topology.Database.MemoryMB
			cpus = t.topology.Database.CPUs
		} else if memMB > 0 {
			memMB /= 2
		}
		ft := t.topology.FaultTolerance
		if ft == "" {
			ft = "none"
		}

		// Render (or accept) the database-node yaml. Falls back to the storage
		// hosts list for placeholder substitution.
		body := t.overrides["ydb.yaml:database"]
		if body == "" {
			body = dbconfig.RenderYDBDatabaseConf(dbconfig.RenderYDBDatabaseConfOpts{
				HostCount:         len(staticHosts),
				HostLocations:     staticLocations,
				DiskPath:          "/ydb_data",
				BlockDevicePaths:  blockDevicePaths,
				CPUs:              cpus,
				MemoryMB:          memMB,
				FaultTolerance:    ft,
				FailureDomainType: t.topology.FailureDomainType,
				DefaultDiskType:   t.topology.DefaultDiskType,
			})
		}
		body = dbconfig.SubstituteYDBHostPlaceholders(body, staticHosts)

		startScript := fmt.Sprintf(`set -e
systemctl stop ydbd-database 2>/dev/null; systemctl reset-failed ydbd-database 2>/dev/null
echo "starting YDB dynamic (database) node..."
systemd-run --unit=ydbd-database --uid=ydb --gid=ydb `+
			`--setenv=LD_LIBRARY_PATH=/opt/ydb/lib `+
			`/opt/ydb/bin/ydbd server `+
			`--yaml-config %s `+
			`--grpc-port 2136 --ic-port 19002 --mon-port 8766 `+
			`%s`+
			`--tenant %s%s`, dbConfPath, pgwireFlag, dbPath, brokerFlags.String())

		readyScript := `if (for i in $(seq 1 60); do (echo > /dev/tcp/localhost/2136) 2>/dev/null && exit 0; sleep 1; done; exit 1); then
  echo "YDB database node started"
else
  echo "ydbd-database did not start" >&2
  exit 1
fi`

		cmds := []agent.Command{
			writeFile("start_ydb_db", dbConfPath, body),
			runCmd("start_ydb_db", startScript),
			runCmd("start_ydb_db", readyScript),
		}

		wg.Add(1)
		go func(idx int, tgt agent.Target, c []agent.Command) {
			defer wg.Done()
			errs[idx] = sendSeq(nc, t.client, tgt, c...)
		}(i, target, cmds)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func ydbStorageGroups(topology *types.YDBTopology) int {
	if topology != nil && topology.StorageGroups > 0 {
		return topology.StorageGroups
	}
	return 1
}

func ydbStoragePoolKind(topology *types.YDBTopology) string {
	if topology != nil && topology.DefaultDiskType != "" {
		return strings.ToLower(topology.DefaultDiskType)
	}
	return "ssd"
}

// allBlockDevicePaths returns the in-guest device path for every secondary
// disk that has a DeviceName, preserving order. Yandex Cloud surfaces
// secondary disks at /dev/disk/by-id/virtio-<DeviceName>, so DeviceName must
// be set. Returns nil when there are no eligible disks (file-backed pdisk
// fallback in the agent).
func allBlockDevicePaths(disks []types.SecondaryDisk) []string {
	var paths []string
	for _, d := range disks {
		if d.DeviceName != "" {
			paths = append(paths, "/dev/disk/by-id/virtio-"+d.DeviceName)
		}
	}
	return paths
}
