package types

import (
	"encoding/json"
	"fmt"
)

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
	Postgres   *PostgresTopology   `json:"postgres,omitempty"`
	MySQL      *MySQLTopology      `json:"mysql,omitempty"`
	MariaDB    *MySQLTopology      `json:"mariadb,omitempty"` // shape mirrors MySQL
	Picodata   *PicodataTopology   `json:"picodata,omitempty"`
	YDB        *YDBTopology        `json:"ydb,omitempty"`
	YDBManaged *YDBManagedTopology `json:"ydb_managed,omitempty"`
	Cockroach  *CockroachTopology  `json:"cockroach,omitempty"`
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
	case DatabaseMariaDB:
		b, err := json.Marshal(p.MariaDB)
		return string(b), err
	case DatabasePicodata:
		b, err := json.Marshal(p.Picodata)
		return string(b), err
	case DatabaseYDB:
		b, err := json.Marshal(p.YDB)
		return string(b), err
	case DatabaseYDBManaged:
		b, err := json.Marshal(p.YDBManaged)
		return string(b), err
	case DatabaseCockroach:
		b, err := json.Marshal(p.Cockroach)
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
	case DatabaseMariaDB:
		var t MySQLTopology
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return err
		}
		p.MariaDB = &t
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
	case DatabaseYDBManaged:
		var t YDBManagedTopology
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return err
		}
		p.YDBManaged = &t
	case DatabaseCockroach:
		var t CockroachTopology
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return err
		}
		p.Cockroach = &t
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
	// MariaDB reuses the MySQL topology shape — same set of presets, same
	// per-component options. The DbKind switches the install package; the
	// agent's configMySQL writer produces a my.cnf that mariadb-server reads
	// without modification.
	for name, topo := range MySQLPresets {
		t := topo
		out = append(out, Preset{
			Name: "MariaDB " + string(name), Description: describeMySQLPreset(name),
			DbKind: string(DatabaseMariaDB), IsBuiltin: true, MariaDB: &t,
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
	for name, topo := range YDBManagedPresets {
		t := topo
		out = append(out, Preset{
			Name: "YDB Managed " + string(name), Description: describeYDBManagedPreset(name),
			DbKind: string(DatabaseYDBManaged), IsBuiltin: true, YDBManaged: &t,
		})
	}
	for name, topo := range CockroachPresets {
		t := topo
		out = append(out, Preset{
			Name: "CockroachDB " + string(name), Description: describeCockroachPreset(name),
			DbKind: string(DatabaseCockroach), IsBuiltin: true, Cockroach: &t,
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
	YDBSingle        YDBPreset = "single"
	YDBMirror3DC3x32 YDBPreset = "mirror3dc-3x32"
	YDBMirror3DC9x32 YDBPreset = "mirror3dc-9x32"
	YDBMirror3DC3x64 YDBPreset = "mirror3dc-3x64"
)

// Legacy YDB presets use a 64 vCPU / 128 GB RAM node flavor with a 50 GB
// boot disk on plain network-ssd (cheap, just OS + binaries), plus a raw
// block device on storage-role nodes that becomes the YDB pdisk. io-m3 is
// the higher-IOPS replicated SSD class — only used for the pdisk where
// throughput matters. Compute-only nodes don't get the secondary device.
//
// Boot disk type is set explicitly so the topology JSON shows it next to
// disk_gb; that keeps "I'll bump disk_type to io-m3 for performance" from
// silently turning the boot disk into a 50 GB io-m3 (invalid: io-m3 sizes
// must be multiples of 93 GiB).
const (
	ydbNodeCPUs         = 64
	ydbNodeMemoryMB     = 131072 // 128 GiB
	ydbNodeBootDiskGB   = 50
	ydbNodeBootDiskType = "network-ssd"
	ydbStoragePdiskGB   = 558        // 6 × 93 GiB — smallest valid io-m3 size at or above 500 GB
	ydbStorageDevice    = "ydb-data" // virtio device_name → /dev/disk/by-id/virtio-ydb-data
	ydbStoragePdiskType = "network-ssd-io-m3"
)

func ydbStorageNodes(count int) MachineSpec {
	return MachineSpec{
		Role: RoleDatabase, Count: count,
		CPUs: ydbNodeCPUs, MemoryMB: ydbNodeMemoryMB,
		DiskGB:   ydbNodeBootDiskGB,
		DiskType: ydbNodeBootDiskType,
		SecondaryDisks: []SecondaryDisk{{
			DeviceName: ydbStorageDevice,
			SizeGB:     ydbStoragePdiskGB,
			Type:       ydbStoragePdiskType,
		}},
	}
}

func ydbTargetStorageNodes() MachineSpec {
	disks := make([]SecondaryDisk, 0, 3)
	for i := 0; i < 3; i++ {
		disks = append(disks, SecondaryDisk{
			DeviceName: fmt.Sprintf("%s-%d", ydbStorageDevice, i),
			SizeGB:     930,
			Type:       ydbStoragePdiskType,
		})
	}
	return MachineSpec{
		Role: RoleDatabase, Count: 3,
		CPUs: 16, MemoryMB: 32768,
		DiskGB:         ydbNodeBootDiskGB,
		DiskType:       ydbNodeBootDiskType,
		SecondaryDisks: disks,
		Placement:      &PlacementSpec{Strategy: "round-robin"},
	}
}

func ydbTargetDatabaseNodes(count, cpus, memoryMB int) *MachineSpec {
	return &MachineSpec{
		Role: RoleDatabase, Count: count,
		CPUs: cpus, MemoryMB: memoryMB,
		DiskGB:    ydbNodeBootDiskGB,
		DiskType:  ydbNodeBootDiskType,
		Placement: &PlacementSpec{Strategy: "round-robin"},
	}
}

func ydbMirror3DCTarget(database *MachineSpec) YDBTopology {
	return YDBTopology{
		Storage:           ydbTargetStorageNodes(),
		Database:          database,
		FaultTolerance:    "mirror-3-dc",
		FailureDomainType: "disk",
		DefaultDiskType:   "SSD",
		StorageGroups:     8,
		DatabasePath:      "/Root/testdb",
	}
}

var YDBPresets = map[YDBPreset]YDBTopology{
	YDBSingle: {
		Storage:        ydbStorageNodes(1),
		FaultTolerance: "none",
		DatabasePath:   "/Root/testdb",
		AutoSizePdisks: true,
	},
	YDBMirror3DC3x32: ydbMirror3DCTarget(ydbTargetDatabaseNodes(3, 32, 65536)),
	YDBMirror3DC9x32: ydbMirror3DCTarget(ydbTargetDatabaseNodes(9, 32, 65536)),
	YDBMirror3DC3x64: ydbMirror3DCTarget(ydbTargetDatabaseNodes(3, 64, 131072)),
}

func describeYDBPreset(p YDBPreset) string {
	switch p {
	case YDBSingle:
		return "1 universal node (storage + compute on one box)"
	case YDBMirror3DC3x32:
		return "Target perf topology: mirror-3-dc, 3×32 vCPU compute, 3×16 vCPU storage, 9 io-m3 pdisks"
	case YDBMirror3DC9x32:
		return "Target perf topology: mirror-3-dc, 9×32 vCPU compute, 3×16 vCPU storage, 9 io-m3 pdisks"
	case YDBMirror3DC3x64:
		return "Target perf topology: mirror-3-dc, 3×64 vCPU compute, 3×16 vCPU storage, 9 io-m3 pdisks"
	default:
		return string(p)
	}
}

// YDBManagedPreset identifies a Yandex Cloud Managed YDB topology preset.
type YDBManagedPreset string

const (
	YDBManagedServerless YDBManagedPreset = "serverless"
	YDBManagedDedicated  YDBManagedPreset = "dedicated"
)

// ydbManagedClient is the default stroppy-runner spec for managed YDB
// presets. The VM gets an attached service account with ydb.editor by the
// terraform module so the patched stroppy ydb driver can pull SA token + CA
// from the YC metadata service.
func ydbManagedClient() MachineSpec {
	return MachineSpec{
		Role:     RoleStroppy,
		Count:    1,
		CPUs:     8,
		MemoryMB: 16384,
		DiskGB:   50,
		DiskType: "network-ssd",
	}
}

// YDBManagedPresets contains the built-in Managed YDB topology presets.
// Endpoint and DatabasePath are populated from terraform output at run
// time, not stored in the preset.
var YDBManagedPresets = map[YDBManagedPreset]YDBManagedTopology{
	YDBManagedServerless: {
		Type:   YDBManagedKindServerless,
		Client: ydbManagedClient(),
	},
	YDBManagedDedicated: {
		Type:             YDBManagedKindDedicated,
		ResourcePresetID: "medium",
		StorageGroups:    1,
		StorageType:      "ssd",
		Client:           ydbManagedClient(),
	},
}

func describeYDBManagedPreset(p YDBManagedPreset) string {
	switch p {
	case YDBManagedServerless:
		return "Yandex Cloud Managed YDB — serverless (pay-per-request, grpcs only)"
	case YDBManagedDedicated:
		return "Yandex Cloud Managed YDB — dedicated medium (1 storage group, ssd)"
	default:
		return string(p)
	}
}

func describeCockroachPreset(p CockroachPreset) string {
	switch p {
	case CockroachSingle:
		return "Single CockroachDB node — dev / smoke runs"
	case CockroachCluster3:
		return "3-node CockroachDB cluster"
	case CockroachCluster6:
		return "6-node CockroachDB cluster (more parallel ranges)"
	default:
		return string(p)
	}
}
