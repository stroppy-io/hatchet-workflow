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
	YDBSingle     YDBPreset = "single"
	YDBUniversal3 YDBPreset = "universal-3"
	YDBSplit33    YDBPreset = "split-3-3"
	YDBSplit63    YDBPreset = "split-6-3"
	YDBSplit36    YDBPreset = "split-3-6"
)

// All YDB nodes share the same flavor for now: 64 vCPU / 128 GB RAM, 50 GB
// boot disk, plus a 500 GB io-m3 raw block device on storage-role nodes
// that becomes the YDB pdisk. Compute-only nodes don't get the secondary
// device. io-m3 is the higher-IOPS replicated SSD class — closer to what
// YDB benchmarks need than the default network-ssd.
const (
	ydbNodeCPUs           = 64
	ydbNodeMemoryMB       = 131072 // 128 GiB
	ydbNodeBootDiskGB     = 50
	ydbStoragePdiskGB     = 500
	ydbStorageDevice      = "ydb-data" // virtio device_name → /dev/disk/by-id/virtio-ydb-data
	ydbStoragePdiskType   = "network-ssd-io-m3"
)

func ydbStorageNodes(count int) MachineSpec {
	return MachineSpec{
		Role: RoleDatabase, Count: count,
		CPUs: ydbNodeCPUs, MemoryMB: ydbNodeMemoryMB, DiskGB: ydbNodeBootDiskGB,
		SecondaryDisks: []SecondaryDisk{{
			DeviceName: ydbStorageDevice,
			SizeGB:     ydbStoragePdiskGB,
			Type:       ydbStoragePdiskType,
		}},
	}
}

func ydbDatabaseNodes(count int) *MachineSpec {
	return &MachineSpec{
		Role: RoleDatabase, Count: count,
		CPUs: ydbNodeCPUs, MemoryMB: ydbNodeMemoryMB, DiskGB: ydbNodeBootDiskGB,
	}
}

var YDBPresets = map[YDBPreset]YDBTopology{
	YDBSingle: {
		Storage:        ydbStorageNodes(1),
		FaultTolerance: "none",
		DatabasePath:   "/Root/testdb",
	},
	YDBUniversal3: {
		Storage:        ydbStorageNodes(3),
		FaultTolerance: "none",
		DatabasePath:   "/Root/testdb",
	},
	YDBSplit33: {
		Storage:        ydbStorageNodes(3),
		Database:       ydbDatabaseNodes(3),
		FaultTolerance: "none",
		DatabasePath:   "/Root/testdb",
	},
	YDBSplit63: {
		Storage:        ydbStorageNodes(6),
		Database:       ydbDatabaseNodes(3),
		FaultTolerance: "none",
		DatabasePath:   "/Root/testdb",
	},
	YDBSplit36: {
		Storage:        ydbStorageNodes(3),
		Database:       ydbDatabaseNodes(6),
		FaultTolerance: "none",
		DatabasePath:   "/Root/testdb",
	},
}

func describeYDBPreset(p YDBPreset) string {
	switch p {
	case YDBSingle:
		return "1 universal node (storage + compute on one box)"
	case YDBUniversal3:
		return "3 universal nodes (storage + compute on each)"
	case YDBSplit33:
		return "Split: 3 storage + 3 database nodes (6 total)"
	case YDBSplit63:
		return "Split: 6 storage + 3 database nodes — storage-heavy (9 total)"
	case YDBSplit36:
		return "Split: 3 storage + 6 database nodes — compute-heavy (9 total)"
	default:
		return string(p)
	}
}
