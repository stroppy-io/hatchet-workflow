package run

import (
	"fmt"
	"strings"
	"sync"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type ydbInstallTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.YDBTopology
	pkg      *types.Package
}

func (t *ydbInstallTask) Execute(nc *dag.NodeContext) error {
	// Install ydbd binaries on every YDB node (storage + compute alike).
	targets := t.state.DBTargets()
	nc.Log().Info("installing YDB on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallYDB,
		Config: agent.YDBInstallConfig{Version: t.version},
	})
}

type ydbConfigTask struct {
	client     agent.Client
	state      *State
	topology   *types.YDBTopology
	overrides  map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "ydb.yaml:storage"
	pgwirePort int               // > 0 → ydbd starts with --pgwire-port (set when run uses ydb-pgwire protocol)
}

func (t *ydbConfigTask) Execute(nc *dag.NodeContext) error {
	// Static (storage) daemon runs on storage nodes only. In combined mode
	// these are also the only YDB nodes, so this matches DBTargets().
	targets := t.state.YDBStorageTargets()
	nc.Log().Info("configuring YDB static nodes")

	hosts := make([]string, len(targets))
	locations := make([]string, len(targets))
	for i, tgt := range targets {
		h := tgt.InternalHost
		if h == "" {
			h = tgt.Host
		}
		hosts[i] = h
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

	// Start all static nodes in parallel — YDB needs all nodes up to form a cluster.
	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for i, target := range targets {
		advHost := target.InternalHost
		if advHost == "" {
			advHost = target.Host
		}
		cfg := agent.YDBStaticConfig{
			Hosts:             hosts,
			HostLocations:     locations,
			InstanceID:        i,
			AdvertiseHost:     advHost,
			DiskPath:          "/ydb_data",
			BlockDevicePaths:  blockDevicePaths,
			DiskGB:            t.topology.Storage.DiskGB,
			MemoryMB:          storageMemMB,
			CPUs:              t.topology.Storage.CPUs,
			PgwirePort:        t.pgwirePort,
			FaultTolerance:    ft,
			FailureDomainType: t.topology.FailureDomainType,
			DefaultDiskType:   t.topology.DefaultDiskType,
			Options:           t.topology.StorageOptions,
			ConfOverride:      t.overrides["ydb.yaml:storage"],
		}
		wg.Add(1)
		go func(idx int, tgt agent.Target, c agent.YDBStaticConfig) {
			defer wg.Done()
			errs[idx] = t.client.Send(nc, tgt, agent.Command{
				Action: agent.ActionConfigYDB, Config: c,
			})
		}(i, target, cfg)
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
	pdiskGB := diskGB - 2
	if pdiskGB < 10 {
		pdiskGB = 10
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
		"pdisk_gb":       fmt.Sprintf("%d", pdiskGB),
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
	client   agent.Client
	state    *State
	topology *types.YDBTopology
}

func (t *ydbInitTask) Execute(nc *dag.NodeContext) error {
	// Cluster init runs once against the static endpoint — first storage node.
	targets := t.state.YDBStorageTargets()
	if len(targets) == 0 {
		return fmt.Errorf("no YDB storage targets for cluster init")
	}

	first := targets[0]
	host := first.InternalHost
	if host == "" {
		host = first.Host
	}

	dbPath := t.topology.DatabasePath
	if dbPath == "" {
		dbPath = "/Root/testdb"
	}

	nc.Log().Info("initializing YDB cluster")
	return t.client.Send(nc, first, agent.Command{
		Action: agent.ActionInitYDB,
		Config: agent.YDBInitConfig{
			StaticEndpoint:  fmt.Sprintf("grpc://%s:2136", host),
			DatabasePath:    dbPath,
			ConfigPath:      "/opt/ydb/cfg/config.yaml",
			StorageGroups:   ydbStorageGroups(t.topology),
			StoragePoolKind: ydbStoragePoolKind(t.topology),
		},
	})
}

type ydbStartDBTask struct {
	client     agent.Client
	state      *State
	topology   *types.YDBTopology
	overrides  map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "ydb.yaml:database"
	pgwirePort int               // > 0 → ydbd starts with --pgwire-port (set when run uses ydb-pgwire protocol)
}

func (t *ydbStartDBTask) Execute(nc *dag.NodeContext) error {
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
		h := tgt.InternalHost
		if h == "" {
			h = tgt.Host
		}
		staticHosts[i] = h
		staticLocations[i] = tgt.Zone
	}

	dbPath := t.topology.DatabasePath
	if dbPath == "" {
		dbPath = "/Root/testdb"
	}

	// Start all dynamic nodes in parallel.
	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for i, target := range targets {
		advHost := target.InternalHost
		if advHost == "" {
			advHost = target.Host
		}
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
		cfg := agent.YDBDatabaseConfig{
			StaticEndpoints:   staticHosts,
			AdvertiseHost:     advHost,
			DatabasePath:      dbPath,
			MemoryMB:          memMB,
			CPUs:              cpus,
			FaultTolerance:    ft,
			FailureDomainType: t.topology.FailureDomainType,
			DefaultDiskType:   t.topology.DefaultDiskType,
			StorageHosts:      staticHosts,
			StorageLocations:  staticLocations,
			BlockDevicePaths:  allBlockDevicePaths(t.topology.Storage.SecondaryDisks),
			PgwirePort:        t.pgwirePort,
			Options:           t.topology.DatabaseOptions,
			ConfOverride:      t.overrides["ydb.yaml:database"],
		}
		wg.Add(1)
		go func(idx int, tgt agent.Target, c agent.YDBDatabaseConfig) {
			defer wg.Done()
			errs[idx] = t.client.Send(nc, tgt, agent.Command{
				Action: agent.ActionStartYDBDB, Config: c,
			})
		}(i, target, cfg)
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
