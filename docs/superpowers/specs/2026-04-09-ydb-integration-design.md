# YDB Integration Design

**Date:** 2026-04-09
**Status:** Approved

## Overview

Add YDB (Yandex Database) as a new database type in stroppy-cloud. YDB has a unique split architecture: **static nodes** (storage layer) and **dynamic nodes** (compute/database layer), requiring a multi-phase orchestration flow.

Stroppy already has a fully implemented YDB driver (`pkg/driver/ydb/`), SQL workload files (`tpcb/ydb.sql`, `tpcc/ydb.sql`), and `ydb-go-sdk/v3` in dependencies. No changes needed on the stroppy side.

## Topology

### Database Kind

`DatabaseYDB DatabaseKind = "ydb"`

### YDBTopology struct

```go
type YDBTopology struct {
    Storage         MachineSpec       `json:"storage"`
    Database        *MachineSpec      `json:"database,omitempty"`   // nil = combined mode
    HAProxy         *MachineSpec      `json:"haproxy,omitempty"`
    FaultTolerance  string            `json:"fault_tolerance"`      // "none", "block-4-2", "mirror-3-dc"
    DatabasePath    string            `json:"database_path"`        // default "/Root/testdb"
    StorageOptions  map[string]string `json:"storage_options,omitempty"`
    DatabaseOptions map[string]string `json:"database_options,omitempty"`
    HAProxyOptions  map[string]string `json:"haproxy_options,omitempty"`
}
```

### Presets

| Preset | Static (storage) | Dynamic (database) | FaultTolerance | Description |
|--------|------------------|--------------------|----------------|-------------|
| ydb-single | 1 node, 2cpu/4GB | nil (combined) | none | Single-node dev mode |
| ydb-cluster | 3 nodes, 4cpu/8GB | 3 nodes, 4cpu/8GB | none | Split storage/compute |
| ydb-scale | 3 nodes, 8cpu/16GB | 6 nodes, 8cpu/16GB | none | Larger split cluster |

Combined mode (Database == nil): static nodes run both storage and database processes on the same machine. The dynamic node process is started on the same host after cluster init.

## DAG Phases

Two new phases added to the DAG for YDB runs:

```
network → machines → install_db → configure_db (start static nodes)
                                       ↓
                                  init_ydb_cluster (blobstorage init + create database)
                                       ↓
                                  start_ydb_database (dynamic nodes or combined-mode db process)
                                       ↓
                          install_monitor → configure_monitor
                          install_stroppy ─────────────────→ run_stroppy → teardown
```

### Phase Constants

```go
PhaseInitYDBCluster   Phase = "init_ydb_cluster"
PhaseStartYDBDatabase Phase = "start_ydb_database"
```

### Phase Dependencies

- `init_ydb_cluster` depends on `configure_db` (all static nodes running)
- `start_ydb_database` depends on `init_ydb_cluster`
- `run_stroppy` depends on `start_ydb_database` (instead of `configure_db`)
- `configure_monitor` depends on `start_ydb_database` (need running DB to scrape)

## Agent Actions

```go
ActionInstallYDB  Action = "install_ydb"   // download ydbd + ydb CLI
ActionConfigYDB   Action = "config_ydb"    // write config, start static node
ActionInitYDB     Action = "init_ydb"      // blobstorage init + create database
ActionStartYDBDB  Action = "start_ydb_db"  // start dynamic node process
```

## Agent Payloads

### YDBInstallConfig

```go
type YDBInstallConfig struct {
    Version string         `json:"version"`
    Package *types.Package `json:"package,omitempty"`
}
```

Install steps:
1. Download `https://binaries.ydb.tech/ydbd-stable-linux-amd64.tar.gz` → `/opt/ydb/`
2. Install YDB CLI: `curl -sSL https://install.ydb.tech/cli | bash`
3. Create system user: `groupadd ydb && useradd ydb -g ydb`
4. Create data directory, set permissions

### YDBStaticConfig

```go
type YDBStaticConfig struct {
    Hosts           []string          `json:"hosts"`            // all static node addresses
    InstanceID      int               `json:"instance_id"`
    AdvertiseHost   string            `json:"advertise_host"`
    DiskPath        string            `json:"disk_path"`        // "/ydb_data" in Docker, real disk on VM
    FaultTolerance  string            `json:"fault_tolerance"`
    Options         map[string]string `json:"options,omitempty"`
}
```

Config steps:
1. Generate `/opt/ydb/cfg/config.yaml` with host_configs, hosts, blob_storage_config sections
2. Prepare disk (create data file in Docker, use partition on VM)
3. Clear disk with `ydbd admin bs disk obliterate`
4. Start static node via `systemd-run --unit=ydbd-storage`

### YDBInitConfig

```go
type YDBInitConfig struct {
    StaticEndpoint string `json:"static_endpoint"` // grpc://<host>:2136
    DatabasePath   string `json:"database_path"`   // /Root/testdb
}
```

Init steps (runs on ONE static node):
1. Wait for static node gRPC to be ready
2. `ydbd admin blobstorage config init --yaml-file /opt/ydb/cfg/config.yaml`
3. `ydbd admin database /Root/testdb create ssd:1`

### YDBDatabaseConfig

```go
type YDBDatabaseConfig struct {
    StaticEndpoints []string          `json:"static_endpoints"` // node-broker addresses
    AdvertiseHost   string            `json:"advertise_host"`
    DatabasePath    string            `json:"database_path"`    // /Root/testdb
    Options         map[string]string `json:"options,omitempty"`
}
```

Start steps:
1. Start dynamic node via `systemd-run --unit=ydbd-database` with `--tenant` and `--node-broker` flags
2. Wait for gRPC port 2136 to accept connections

## Ports

| Service | Port | Description |
|---------|------|-------------|
| gRPC | 2136 | Client connections (unencrypted, dev mode) |
| Interconnect (static) | 19001 | Inter-node communication |
| Interconnect (dynamic) | 19002 | Inter-node communication |
| Monitoring (static) | 8765 | Web UI + Prometheus metrics |
| Monitoring (dynamic) | 8766 | Prometheus metrics |

No TLS for Docker/dev mode. TLS can be added later for YC VM deployments.

## Stroppy Connection

- Driver type: `ydb`
- URL format: `grpc://<host>:2136/Root/testdb`
- Through HAProxy: `grpc://<proxy>:2136/Root/testdb`
- Default insert method: `plain_bulk`

In task_infra.go, DB port = 2136. Proxy port = 2136 (HAProxy TCP passthrough).

## Monitoring

### vmagent Scrape Config

YDB exports Prometheus metrics natively — no separate exporter needed.

```yaml
- job_name: ydb
  metrics_path: /counters/counters=ydb/prometheus
  static_configs:
    - targets: ["<node>:8765"]  # static nodes
    - targets: ["<node>:8766"]  # dynamic nodes
```

Added to configMonitor in executor.go alongside existing node_exporter/postgres_exporter/mysqld_exporter jobs.

### Metric Queries

```go
func ydbMetrics() []MetricDef {
    return []MetricDef{
        {Name: "DB Requests/s", Key: "db_qps",
         Query: `sum(rate(api_grpc_request_count_total{%s}[5m]))`, Unit: "req/s"},
        {Name: "DB Request Errors/s", Key: "db_errors",
         Query: `sum(rate(api_grpc_request_errors_total{%s}[5m]))`, Unit: "err/s"},
        {Name: "DB Active Sessions", Key: "db_sessions",
         Query: `sum(table_service_active_sessions{%s})`, Unit: ""},
        {Name: "DB Tablet Count", Key: "db_tablets",
         Query: `sum(ydb_tablets_count{%s})`, Unit: ""},
    }
}
```

Exact metric names to be verified against actual YDB Prometheus output during testing.

## File Changes

### New Files

| File | Description |
|------|-------------|
| `internal/domain/agent/setup_ydb.go` | Install/Config/Init/StartDB payload structs |
| `internal/domain/run/task_ydb.go` | ydbInstallTask, ydbConfigTask, ydbInitTask, ydbStartDBTask |
| `deployments/grafana/dashboards/stroppy-ydb.json` | Grafana dashboard |

### Modified Files

| File | Change |
|------|--------|
| `internal/domain/types/run.go` | DatabaseYDB constant, YDBTopology struct, PhaseInitYDBCluster, PhaseStartYDBDatabase |
| `internal/domain/types/db_defaults.go` | YDBDefaults() function |
| `internal/domain/types/presets.go` | 3 YDB presets in BuiltinPresets(), ParseTopology/TopologyJSON cases |
| `internal/domain/agent/protocol.go` | 4 new Action constants |
| `internal/domain/agent/executor.go` | 4 new handler methods: installYDB, configYDB, initYDB, startYDBDB |
| `internal/domain/run/builder.go` | dbTasks() case for YDB, addYDBPhases() helper, phase wiring |
| `internal/domain/run/task_infra.go` | Port 2136 in dockerMachines/yandexMachines |
| `internal/domain/run/task_stroppy.go` | YDB driver URL construction |
| `internal/domain/run/task_proxy.go` | HAProxy TCP passthrough on port 2136 |
| `internal/domain/run/validate.go` | YDB script compatibility |
| `internal/domain/run/machines.go` | YDB topology → MachineSpec extraction |
| `internal/domain/metrics/queries.go` | ydbMetrics() function, MetricsForDB case |
| `internal/domain/api/server.go` | Grafana dashboard reference |
| `web/src/api/types.ts` | YDBTopology interface, "ydb" in DatabaseKind |
| `web/src/pages/NewRun.tsx` | YDB in DB_KINDS, DB_VERSIONS, DB_META |
| `web/src/pages/PresetDesigner.tsx` | YDB topology editor with storage/database/haproxy sections |
| `web/src/components/LogStream.tsx` | Action labels for YDB phases |

## Out of Scope

- TLS/certificate management (dev mode only, no encryption)
- Multi-datacenter fault tolerance (block-4-2 / mirror-3-dc topologies defined in struct but not orchestrated)
- YDB authentication (root without password for dev)
- Disk partitioning on YC VMs (use file-based storage initially)
