package types

import "encoding/json"

// Preset describes a database topology preset.
// Stored in the presets table; one row = one topology template.
type Preset struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DbKind      string `json:"db_kind"`
	IsBuiltin   bool   `json:"is_builtin"`

	// Exactly one topology field is set, matching DbKind.
	Postgres *PostgresTopology `json:"postgres,omitempty"`
	MySQL    *MySQLTopology    `json:"mysql,omitempty"`
	Picodata *PicodataTopology `json:"picodata,omitempty"`
	YDB      *YDBTopology      `json:"ydb,omitempty"`
}

// TopologyJSON serializes the active topology field to JSON for DB storage.
func (p *Preset) TopologyJSON() (string, error) {
	switch DatabaseKind(p.DbKind) {
	case DatabasePostgres:
		b, err := json.Marshal(p.Postgres)
		return string(b), err
	case DatabaseMySQL:
		b, err := json.Marshal(p.MySQL)
		return string(b), err
	case DatabasePicodata:
		b, err := json.Marshal(p.Picodata)
		return string(b), err
	case DatabaseYDB:
		b, err := json.Marshal(p.YDB)
		return string(b), err
	default:
		return "", nil
	}
}

// ParseTopology deserializes a JSON string into the correct topology field based on DbKind.
func (p *Preset) ParseTopology(raw string) error {
	switch DatabaseKind(p.DbKind) {
	case DatabasePostgres:
		var t PostgresTopology
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return err
		}
		p.Postgres = &t
	case DatabaseMySQL:
		var t MySQLTopology
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return err
		}
		p.MySQL = &t
	case DatabasePicodata:
		var t PicodataTopology
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return err
		}
		p.Picodata = &t
	case DatabaseYDB:
		var t YDBTopology
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return err
		}
		p.YDB = &t
	}
	return nil
}

// BuiltinPresets returns the default topology presets for all supported databases.
// Used to seed new tenants.
func BuiltinPresets() []Preset {
	var out []Preset

	for name, topo := range PostgresPresets {
		t := topo
		out = append(out, Preset{
			Name: "PostgreSQL " + string(name), Description: describePostgresPreset(name),
			DbKind: string(DatabasePostgres), IsBuiltin: true, Postgres: &t,
		})
	}
	for name, topo := range MySQLPresets {
		t := topo
		out = append(out, Preset{
			Name: "MySQL " + string(name), Description: describeMySQLPreset(name),
			DbKind: string(DatabaseMySQL), IsBuiltin: true, MySQL: &t,
		})
	}
	for name, topo := range PicodataPresets {
		t := topo
		out = append(out, Preset{
			Name: "Picodata " + string(name), Description: describePicodataPreset(name),
			DbKind: string(DatabasePicodata), IsBuiltin: true, Picodata: &t,
		})
	}
	for name, topo := range YDBPresets {
		t := topo
		out = append(out, Preset{
			Name: "YDB " + string(name), Description: describeYDBPreset(name),
			DbKind: string(DatabaseYDB), IsBuiltin: true, YDB: &t,
		})
	}

	return out
}

func describePostgresPreset(p PostgresPreset) string {
	switch p {
	case PostgresSingle:
		return "Single PostgreSQL instance"
	case PostgresHA:
		return "PostgreSQL with Patroni, HAProxy, PgBouncer, synchronous replication"
	case PostgresScale:
		return "PostgreSQL with 4 replicas, 2 HAProxy nodes, full HA stack"
	default:
		return ""
	}
}

func describeMySQLPreset(p MySQLPreset) string {
	switch p {
	case MySQLSingle:
		return "Single MySQL instance"
	case MySQLReplica:
		return "MySQL with semi-synchronous replication and ProxySQL"
	case MySQLGroup:
		return "MySQL with Group Replication and ProxySQL"
	default:
		return ""
	}
}

func describePicodataPreset(p PicodataPreset) string {
	switch p {
	case PicodataSingle:
		return "Single Picodata instance"
	case PicodataCluster:
		return "Picodata with 3 instances, 3 shards, HAProxy"
	case PicodataScale:
		return "Picodata with 6 instances, multi-tier deployment"
	default:
		return ""
	}
}

type YDBPreset string

const (
	YDBSingle  YDBPreset = "single"
	YDBCluster YDBPreset = "cluster"
	YDBScale   YDBPreset = "scale"
)

var YDBPresets = map[YDBPreset]YDBTopology{
	YDBSingle: {
		Storage:        MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 80},
		FaultTolerance: "none",
		DatabasePath:   "/Root/testdb",
	},
	YDBCluster: {
		Storage:        MachineSpec{Role: RoleDatabase, Count: 3, CPUs: 4, MemoryMB: 8192, DiskGB: 100},
		Database:       &MachineSpec{Role: RoleDatabase, Count: 3, CPUs: 4, MemoryMB: 8192, DiskGB: 50},
		FaultTolerance: "none",
		DatabasePath:   "/Root/testdb",
	},
	YDBScale: {
		Storage:        MachineSpec{Role: RoleDatabase, Count: 3, CPUs: 8, MemoryMB: 16384, DiskGB: 200},
		Database:       &MachineSpec{Role: RoleDatabase, Count: 6, CPUs: 8, MemoryMB: 16384, DiskGB: 50},
		FaultTolerance: "none",
		DatabasePath:   "/Root/testdb",
	},
}

func describeYDBPreset(p YDBPreset) string {
	switch p {
	case YDBSingle:
		return "Single-node YDB (combined storage+compute)"
	case YDBCluster:
		return "YDB cluster with 3 storage + 3 database nodes"
	case YDBScale:
		return "YDB scale cluster with 3 storage + 6 database nodes"
	default:
		return string(p)
	}
}
