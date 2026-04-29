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

	// At least one topology must be set (or preset_id).
	if cfg.Database.Postgres == nil && cfg.Database.MySQL == nil && cfg.Database.MariaDB == nil && cfg.Database.Picodata == nil && cfg.Database.YDB == nil && cfg.Database.Cockroach == nil && cfg.PresetID == "" {
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
		{types.DatabasePostgres, cfg.Database.MySQL != nil || cfg.Database.MariaDB != nil || cfg.Database.Picodata != nil || cfg.Database.YDB != nil || cfg.Database.Cockroach != nil, "postgres"},
		{types.DatabaseMySQL, cfg.Database.Postgres != nil || cfg.Database.MariaDB != nil || cfg.Database.Picodata != nil || cfg.Database.YDB != nil || cfg.Database.Cockroach != nil, "mysql"},
		{types.DatabaseMariaDB, cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.Picodata != nil || cfg.Database.YDB != nil || cfg.Database.Cockroach != nil, "mariadb"},
		{types.DatabasePicodata, cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.MariaDB != nil || cfg.Database.YDB != nil || cfg.Database.Cockroach != nil, "picodata"},
		{types.DatabaseYDB, cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.MariaDB != nil || cfg.Database.Picodata != nil || cfg.Database.Cockroach != nil, "ydb"},
		{types.DatabaseCockroach, cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.MariaDB != nil || cfg.Database.Picodata != nil || cfg.Database.YDB != nil, "cockroach"},
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
	if script != "" {
		supported := types.ScriptCompat[types.KindProtocolKey{Kind: cfg.Database.Kind, Protocol: protocol}]
		if len(supported) > 0 {
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

	return nil
}
