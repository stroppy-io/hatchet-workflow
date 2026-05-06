package run

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// Script compatibility now lives in types.ScriptCompat, keyed by
// (DatabaseKind, Protocol). The old scriptDBSupport map keyed only on kind
// and assumed scripts were portable across protocols — which broke the
// moment we wanted to differentiate native YDB from YDB pgwire (different
// SQL feature ceilings, different stroppy script variants).

// ValidateConfig checks RunConfig semantics before building the DAG.
func ValidateConfig(cfg types.RunConfig) error {
	// Database kind must be set.
	if cfg.Database.Kind == "" {
		return fmt.Errorf("database.kind is required")
	}

	// External DB short-circuits topology requirements — the user supplies
	// the endpoint themselves so there's no infra to plan.
	if cfg.ExternalDB != nil {
		if cfg.ExternalDB.Endpoint == "" {
			return fmt.Errorf("external_db.endpoint is required")
		}
	} else if cfg.Database.Postgres == nil && cfg.Database.MySQL == nil && cfg.Database.MariaDB == nil && cfg.Database.Picodata == nil && cfg.Database.YDB == nil && cfg.Database.YDBManaged == nil && cfg.Database.Cockroach == nil && cfg.PresetID == "" {
		return fmt.Errorf("database topology or preset_id is required")
	}

	// Topology must match database kind. The check is symmetric — flag any
	// non-matching topology pointer that's set; saves a 6-way switch.
	type topoCheck struct {
		ownKind  types.DatabaseKind
		otherSet bool
		label    string
	}
	checks := []topoCheck{
		{types.DatabasePostgres, cfg.Database.MySQL != nil || cfg.Database.MariaDB != nil || cfg.Database.Picodata != nil || cfg.Database.YDB != nil || cfg.Database.YDBManaged != nil || cfg.Database.Cockroach != nil, "postgres"},
		{types.DatabaseMySQL, cfg.Database.Postgres != nil || cfg.Database.MariaDB != nil || cfg.Database.Picodata != nil || cfg.Database.YDB != nil || cfg.Database.YDBManaged != nil || cfg.Database.Cockroach != nil, "mysql"},
		{types.DatabaseMariaDB, cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.Picodata != nil || cfg.Database.YDB != nil || cfg.Database.YDBManaged != nil || cfg.Database.Cockroach != nil, "mariadb"},
		{types.DatabasePicodata, cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.MariaDB != nil || cfg.Database.YDB != nil || cfg.Database.YDBManaged != nil || cfg.Database.Cockroach != nil, "picodata"},
		{types.DatabaseYDB, cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.MariaDB != nil || cfg.Database.Picodata != nil || cfg.Database.YDBManaged != nil || cfg.Database.Cockroach != nil, "ydb"},
		{types.DatabaseYDBManaged, cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.MariaDB != nil || cfg.Database.Picodata != nil || cfg.Database.YDB != nil || cfg.Database.Cockroach != nil, "ydb-managed"},
		{types.DatabaseCockroach, cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.MariaDB != nil || cfg.Database.Picodata != nil || cfg.Database.YDB != nil || cfg.Database.YDBManaged != nil, "cockroach"},
	}
	for _, c := range checks {
		if cfg.Database.Kind == c.ownKind && c.otherSet {
			return fmt.Errorf("database.kind is %s but non-%s topology is set", c.label, c.label)
		}
	}

	// Script vs (DB kind, protocol) compatibility. Protocol defaults to the
	// kind's first supported entry when the run config didn't set one.
	script := cfg.Stroppy.Script
	if script == "" {
		script = cfg.Stroppy.Workload // backward compat
	}
	protocol := cfg.Stroppy.Protocol
	if protocol == "" {
		protocol = types.DefaultProtocol(cfg.Database.Kind)
	}
	if protocol != "" && !types.KindSupportsProtocol(cfg.Database.Kind, protocol) {
		return fmt.Errorf("protocol %q is not supported by database %q", protocol, cfg.Database.Kind)
	}
	if script != "" && knownScript(script) {
		supported := types.ScriptCompat[types.KindProtocolKey{Kind: cfg.Database.Kind, Protocol: protocol}]
		found := false
		for _, s := range supported {
			if s == script {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("script %q is not compatible with database %q on protocol %q (supported: %s)",
				script, cfg.Database.Kind, protocol, strings.Join(supported, ", "))
		}
	}

	// Stroppy basic validation.
	if cfg.Stroppy.Duration != "" {
		d := cfg.Stroppy.Duration
		// Must end with s, m, or h.
		if len(d) < 2 {
			return fmt.Errorf("invalid duration %q", d)
		}
		suffix := d[len(d)-1]
		if suffix != 's' && suffix != 'm' && suffix != 'h' {
			return fmt.Errorf("duration %q must end with s, m, or h", d)
		}
	}

	if cfg.Stroppy.VUs < 0 {
		return fmt.Errorf("vus must be >= 0")
	}
	if cfg.Stroppy.Iterations < 0 {
		return fmt.Errorf("iterations must be >= 0")
	}
	switch cfg.Stroppy.K6Mode {
	case "", "duration", "iterations":
	default:
		return fmt.Errorf("k6_mode must be either duration or iterations")
	}
	if cfg.Stroppy.PoolSize < 0 {
		return fmt.Errorf("pool_size must be >= 0")
	}
	if cfg.Stroppy.ScaleFactor < 0 {
		return fmt.Errorf("scale_factor must be >= 0")
	}

	// Machine specs basic checks.
	if cfg.Stroppy.Machine != nil {
		m := cfg.Stroppy.Machine
		if m.CPUs < 1 {
			return fmt.Errorf("stroppy machine cpus must be >= 1")
		}
		if m.MemoryMB < 512 {
			return fmt.Errorf("stroppy machine memory must be >= 512 MB")
		}
		if m.DiskGB < 10 {
			return fmt.Errorf("stroppy machine disk must be >= 10 GB")
		}
	}

	if cfg.Database.Kind == types.DatabaseYDB && cfg.Database.YDB != nil {
		if err := validateYDBTopology(cfg.Database.YDB); err != nil {
			return err
		}
	}

	return nil
}

func validateYDBTopology(t *types.YDBTopology) error {
	switch t.FaultTolerance {
	case "", "none", "block-4-2", "mirror-3-dc":
	default:
		return fmt.Errorf("ydb.fault_tolerance must be one of none, block-4-2, mirror-3-dc")
	}
	switch t.FailureDomainType {
	case "", "disk":
	default:
		return fmt.Errorf("ydb.failure_domain_type must be empty or disk")
	}
	if t.DefaultDiskType != "" {
		switch t.DefaultDiskType {
		case "SSD", "NVME", "ROT":
		default:
			return fmt.Errorf("ydb.default_disk_type must be SSD, NVME, or ROT")
		}
	}
	if t.StorageGroups < 0 {
		return fmt.Errorf("ydb.storage_groups must be >= 0")
	}
	if err := validatePlacement("ydb.storage.placement", t.Storage.Placement); err != nil {
		return err
	}
	if err := validateIOM3Disk("ydb.storage.boot", t.Storage.DiskGB, t.Storage.DiskType); err != nil {
		return err
	}
	for i, d := range t.Storage.SecondaryDisks {
		if err := validateIOM3Disk(fmt.Sprintf("ydb.storage.secondary_disks[%d]", i), d.SizeGB, d.Type); err != nil {
			return err
		}
	}
	if t.Database != nil {
		if err := validatePlacement("ydb.database.placement", t.Database.Placement); err != nil {
			return err
		}
		if err := validateIOM3Disk("ydb.database.boot", t.Database.DiskGB, t.Database.DiskType); err != nil {
			return err
		}
	}
	if t.FaultTolerance == "mirror-3-dc" && t.FailureDomainType == "disk" {
		if t.Storage.Count < 3 {
			return fmt.Errorf("ydb mirror-3-dc disk failure-domain topology requires at least 3 storage nodes")
		}
		if len(t.Storage.SecondaryDisks) < 3 {
			return fmt.Errorf("ydb mirror-3-dc disk failure-domain topology requires at least 3 secondary disks per storage node")
		}
	}
	return nil
}

func validatePlacement(path string, p *types.PlacementSpec) error {
	if p == nil {
		return nil
	}
	switch p.Strategy {
	case "", "single", "round-robin":
	default:
		return fmt.Errorf("%s.strategy must be single or round-robin", path)
	}
	return nil
}

func validateIOM3Disk(path string, sizeGB int, diskType string) error {
	if diskType != "network-ssd-io-m3" || sizeGB <= 0 {
		return nil
	}
	if sizeGB%ydbStorageChunkGB != 0 {
		return fmt.Errorf("%s size must be a multiple of %d GB for network-ssd-io-m3", path, ydbStorageChunkGB)
	}
	return nil
}

func knownScript(script string) bool {
	for _, scripts := range types.ScriptCompat {
		for _, s := range scripts {
			if s == script {
				return true
			}
		}
	}
	return false
}
