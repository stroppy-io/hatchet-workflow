# YDB Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add YDB as a new database type in stroppy-cloud with full lifecycle: install, configure static nodes, init cluster, start dynamic nodes, monitor, benchmark.

**Architecture:** YDB has a split storage/compute architecture requiring 4 agent actions (install, config static, init cluster, start database) and 2 new DAG phases (init_ydb_cluster, start_ydb_database). Stroppy already has a YDB driver — only stroppy-cloud orchestration needs work.

**Tech Stack:** Go (backend), React/TypeScript (frontend), YDB ydbd binary, VictoriaMetrics (monitoring), virtua (log virtualization)

**Spec:** `docs/superpowers/specs/2026-04-09-ydb-integration-design.md`

---

### Task 1: Type Definitions — DatabaseKind, Topology, Phases

**Files:**
- Modify: `internal/domain/types/run.go`

- [ ] **Step 1: Add DatabaseYDB constant**

In `internal/domain/types/run.go`, add to the DatabaseKind constants block:

```go
const (
	DatabasePostgres DatabaseKind = "postgres"
	DatabaseMySQL    DatabaseKind = "mysql"
	DatabasePicodata DatabaseKind = "picodata"
	DatabaseYDB      DatabaseKind = "ydb"
)
```

- [ ] **Step 2: Add YDB Phase constants**

Add two new phases to the Phase constants block:

```go
	PhaseInitYDBCluster   Phase = "init_ydb_cluster"
	PhaseStartYDBDatabase Phase = "start_ydb_database"
```

- [ ] **Step 3: Add YDBTopology struct**

Add after PicodataTopology:

```go
// YDBTopology describes a YDB cluster layout.
type YDBTopology struct {
	Storage         MachineSpec       `json:"storage"`                    // static (storage) nodes
	Database        *MachineSpec      `json:"database,omitempty"`         // dynamic (compute) nodes; nil = combined mode
	HAProxy         *MachineSpec      `json:"haproxy,omitempty"`          // optional load balancer
	FaultTolerance  string            `json:"fault_tolerance"`            // "none", "block-4-2", "mirror-3-dc"
	DatabasePath    string            `json:"database_path"`              // default "/Root/testdb"
	StorageOptions  map[string]string `json:"storage_options,omitempty"`
	DatabaseOptions map[string]string `json:"database_options,omitempty"`
	HAProxyOptions  map[string]string `json:"haproxy_options,omitempty"`
}
```

- [ ] **Step 4: Add YDB field to DatabaseConfig**

```go
type DatabaseConfig struct {
	Kind     DatabaseKind      `json:"kind"`
	Version  string            `json:"version"`
	Postgres *PostgresTopology `json:"postgres,omitempty"`
	MySQL    *MySQLTopology    `json:"mysql,omitempty"`
	Picodata *PicodataTopology `json:"picodata,omitempty"`
	YDB      *YDBTopology      `json:"ydb,omitempty"`
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`

---

### Task 2: Defaults and Presets

**Files:**
- Modify: `internal/domain/types/db_defaults.go`
- Modify: `internal/domain/types/presets.go`

- [ ] **Step 1: Add YDBDefaults**

In `db_defaults.go`, add:

```go
// YDBDefaults returns default YDB configuration options by version.
func YDBDefaults(version string) map[string]string {
	return map[string]string{
		"fault_tolerance": "none",
		"database_path":   "/Root/testdb",
	}
}
```

- [ ] **Step 2: Add YDB preset types and map**

In `presets.go`, add the preset type, map, and describe function:

```go
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
```

- [ ] **Step 3: Add YDB field to Preset struct**

Add `YDB *YDBTopology` field to the Preset struct (alongside Postgres/MySQL/Picodata fields).

- [ ] **Step 4: Update TopologyJSON**

Add case to TopologyJSON switch:

```go
	case DatabaseYDB:
		b, err := json.Marshal(p.YDB)
		return string(b), err
```

- [ ] **Step 5: Update ParseTopology**

Add case to ParseTopology switch:

```go
	case DatabaseYDB:
		var t YDBTopology
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return err
		}
		p.YDB = &t
```

- [ ] **Step 6: Update BuiltinPresets**

Add YDB presets loop:

```go
	for name, topo := range YDBPresets {
		t := topo
		out = append(out, Preset{
			Name: "YDB " + string(name), Description: describeYDBPreset(name),
			DbKind: string(DatabaseYDB), IsBuiltin: true, YDB: &t,
		})
	}
```

- [ ] **Step 7: Verify build**

Run: `go build ./...`

---

### Task 3: Agent Protocol and Payloads

**Files:**
- Modify: `internal/domain/agent/protocol.go`
- Create: `internal/domain/agent/setup_ydb.go`

- [ ] **Step 1: Add YDB action constants**

In `protocol.go`, add to the Action constants:

```go
	ActionInstallYDB  Action = "install_ydb"
	ActionConfigYDB   Action = "config_ydb"
	ActionInitYDB     Action = "init_ydb"
	ActionStartYDBDB  Action = "start_ydb_db"
```

- [ ] **Step 2: Create setup_ydb.go**

Create `internal/domain/agent/setup_ydb.go`:

```go
package agent

// YDBInstallConfig is the agent payload for YDB binary installation.
type YDBInstallConfig struct {
	Version string `json:"version"`
}

// YDBStaticConfig is the agent payload for starting a YDB static (storage) node.
type YDBStaticConfig struct {
	Hosts          []string          `json:"hosts"`           // all static node addresses
	InstanceID     int               `json:"instance_id"`
	AdvertiseHost  string            `json:"advertise_host"`
	DiskPath       string            `json:"disk_path"`       // "/ydb_data" in Docker, real disk on VM
	FaultTolerance string            `json:"fault_tolerance"`
	Options        map[string]string `json:"options,omitempty"`
}

// YDBInitConfig is the agent payload for cluster initialization (runs on one static node).
type YDBInitConfig struct {
	StaticEndpoint string `json:"static_endpoint"` // grpc://<host>:2136
	DatabasePath   string `json:"database_path"`   // /Root/testdb
	ConfigPath     string `json:"config_path"`     // /opt/ydb/cfg/config.yaml
}

// YDBDatabaseConfig is the agent payload for starting a YDB dynamic (database) node.
type YDBDatabaseConfig struct {
	StaticEndpoints []string          `json:"static_endpoints"` // node-broker addresses
	AdvertiseHost   string            `json:"advertise_host"`
	DatabasePath    string            `json:"database_path"`    // /Root/testdb
	Options         map[string]string `json:"options,omitempty"`
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

---

### Task 4: Agent Executor — Install & Config Handlers

**Files:**
- Modify: `internal/domain/agent/executor.go`

- [ ] **Step 1: Add dispatch cases in Run()**

Add to the switch in `Run()` method (before `default:`):

```go
	case ActionInstallYDB:
		err = e.installYDB(ctx, cmd)
	case ActionConfigYDB:
		err = e.configYDB(ctx, cmd)
	case ActionInitYDB:
		err = e.initYDB(ctx, cmd)
	case ActionStartYDBDB:
		err = e.startYDBDB(ctx, cmd)
```

- [ ] **Step 2: Implement installYDB**

Add handler method:

```go
// installYDB downloads ydbd server binary and ydb CLI.
func (e *Executor) installYDB(ctx context.Context, cmd Command) error {
	var cfg YDBInstallConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	// Check if already installed (pre-built agent image).
	if _, err := exec.LookPath("/opt/ydb/bin/ydbd"); err == nil {
		e.emitLine("ydbd already installed, skipping download")
		return nil
	}

	// Download ydbd server binary.
	e.emitLine("downloading ydbd server binary...")
	if _, err := e.shell(ctx, `mkdir -p /opt/ydb && curl -fSL https://binaries.ydb.tech/ydbd-stable-linux-amd64.tar.gz | tar -xz --strip-component=1 -C /opt/ydb`); err != nil {
		return fmt.Errorf("download ydbd: %w", err)
	}

	// Install YDB CLI.
	e.emitLine("installing ydb CLI...")
	if _, err := e.shell(ctx, `curl -sSL https://install.ydb.tech/cli | bash`); err != nil {
		return fmt.Errorf("install ydb CLI: %w", err)
	}

	// Create system user.
	e.shell(ctx, "groupadd -f ydb && (id -u ydb &>/dev/null || useradd ydb -g ydb)")
	e.shell(ctx, "mkdir -p /opt/ydb/cfg /ydb_data && chown -R ydb:ydb /ydb_data")

	return nil
}
```

- [ ] **Step 3: Implement configYDB (start static node)**

```go
// configYDB generates YDB config and starts a static (storage) node.
func (e *Executor) configYDB(ctx context.Context, cmd Command) error {
	var cfg YDBStaticConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	diskPath := cfg.DiskPath
	if diskPath == "" {
		diskPath = "/ydb_data"
	}

	// Generate config.yaml.
	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\n")

	// host_configs — disk configuration.
	confBuf.WriteString("host_configs:\n")
	confBuf.WriteString("- drive:\n")
	fmt.Fprintf(&confBuf, "  - path: %s/pdisk.data\n", diskPath)
	confBuf.WriteString("    type: SSD\n")
	confBuf.WriteString("  host_config_id: 1\n")

	// hosts — list all static nodes.
	confBuf.WriteString("hosts:\n")
	for i, host := range cfg.Hosts {
		fmt.Fprintf(&confBuf, "- host: %s\n", host)
		confBuf.WriteString("  host_config_id: 1\n")
		confBuf.WriteString("  walle_location:\n")
		fmt.Fprintf(&confBuf, "    body: %d\n", i+1)
		confBuf.WriteString("    data_center: 'zone-a'\n")
		fmt.Fprintf(&confBuf, "    rack: '%d'\n", i+1)
	}

	// domains_config.
	confBuf.WriteString("domains_config:\n")
	confBuf.WriteString("  domain:\n")
	confBuf.WriteString("  - name: Root\n")
	confBuf.WriteString("    storage_pool_types:\n")
	confBuf.WriteString("    - kind: ssd\n")
	confBuf.WriteString("      pool_config:\n")
	confBuf.WriteString("        box_id: 1\n")

	// erasure based on fault tolerance / node count.
	erasure := "none"
	nHosts := len(cfg.Hosts)
	if nHosts >= 8 {
		erasure = "block-4-2"
	}
	if cfg.FaultTolerance != "" && cfg.FaultTolerance != "none" {
		erasure = cfg.FaultTolerance
	}
	fmt.Fprintf(&confBuf, "        erasure_species: %s\n", erasure)

	confBuf.WriteString("        pdisk_filter:\n")
	confBuf.WriteString("        - property:\n")
	confBuf.WriteString("          - type: SSD\n")
	confBuf.WriteString("        vdisk_kind: Default\n")
	confBuf.WriteString("  state_storage:\n")
	confBuf.WriteString("  - ring:\n")

	// NToSelect = nHosts for single-zone.
	fmt.Fprintf(&confBuf, "      nto_select: %d\n", nHosts)
	confBuf.WriteString("      node:\n")
	for i := range cfg.Hosts {
		fmt.Fprintf(&confBuf, "      - node_id: %d\n", i+1)
	}
	confBuf.WriteString("    ssid: 1\n")

	// blob_storage_config.
	confBuf.WriteString("blob_storage_config:\n")
	confBuf.WriteString("  service_set:\n")
	confBuf.WriteString("    groups:\n")
	confBuf.WriteString("    - erasure_species: ")
	fmt.Fprintf(&confBuf, "%s\n", erasure)
	confBuf.WriteString("      rings:\n")
	for i := range cfg.Hosts {
		confBuf.WriteString("      - fail_domains:\n")
		confBuf.WriteString("        - vdisk_locations:\n")
		fmt.Fprintf(&confBuf, "          - node_id: %d\n", i+1)
		confBuf.WriteString("            pdisk_category: SSD\n")
		confBuf.WriteString("            path: ")
		fmt.Fprintf(&confBuf, "%s/pdisk.data\n", diskPath)
	}

	// actor_system_config — use defaults.
	confBuf.WriteString("actor_system_config:\n")
	confBuf.WriteString("  use_auto_config: true\n")

	// grpc_config.
	confBuf.WriteString("grpc_config:\n")
	confBuf.WriteString("  port: 2136\n")

	confPath := "/opt/ydb/cfg/config.yaml"
	writeScript := fmt.Sprintf("mkdir -p /opt/ydb/cfg && cat > %s << 'YDBCONF'\n%sYDBCONF", confPath, confBuf.String())
	if _, err := e.shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write ydb config: %w", err)
	}

	// Prepare disk data file (emulated disk for Docker).
	e.shell(ctx, fmt.Sprintf("mkdir -p %s && chown -R ydb:ydb %s", diskPath, diskPath))
	// Create 80GB sparse file as emulated PDisk (if not a block device).
	e.shell(ctx, fmt.Sprintf(`test -f %s/pdisk.data || truncate -s 80G %s/pdisk.data`, diskPath, diskPath))
	e.shell(ctx, fmt.Sprintf("chown ydb:ydb %s/pdisk.data", diskPath))

	// Obliterate disk.
	e.emitLine("preparing YDB disk...")
	if _, err := e.shell(ctx, fmt.Sprintf(
		"LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd admin bs disk obliterate %s/pdisk.data", diskPath)); err != nil {
		return fmt.Errorf("obliterate disk: %w", err)
	}

	// Start static node via systemd-run.
	e.shell(ctx, "systemctl stop ydbd-storage 2>/dev/null; systemctl reset-failed ydbd-storage 2>/dev/null")
	e.emitLine("starting YDB static (storage) node...")
	startCmd := fmt.Sprintf(
		`systemd-run --unit=ydbd-storage --uid=ydb --gid=ydb `+
			`--setenv=LD_LIBRARY_PATH=/opt/ydb/lib `+
			`/opt/ydb/bin/ydbd server `+
			`--yaml-config %s `+
			`--grpc-port 2136 --ic-port 19001 --mon-port 8765 `+
			`--node static`,
		confPath)
	if _, err := e.shell(ctx, startCmd); err != nil {
		return fmt.Errorf("start ydbd-storage: %w", err)
	}

	// Wait for gRPC port to be ready.
	if _, err := e.shell(ctx, `for i in $(seq 1 30); do nc -z localhost 2136 && exit 0; sleep 1; done; exit 1`); err != nil {
		return fmt.Errorf("ydbd-storage did not start: %w", err)
	}

	e.emitLine("YDB static node started")
	return nil
}
```

- [ ] **Step 4: Implement initYDB (cluster initialization)**

```go
// initYDB initializes the YDB cluster — blobstorage config + create database.
// Runs on ONE static node after all static nodes are up.
func (e *Executor) initYDB(ctx context.Context, cmd Command) error {
	var cfg YDBInitConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	endpoint := cfg.StaticEndpoint
	if endpoint == "" {
		endpoint = "grpc://localhost:2136"
	}
	confPath := cfg.ConfigPath
	if confPath == "" {
		confPath = "/opt/ydb/cfg/config.yaml"
	}
	dbPath := cfg.DatabasePath
	if dbPath == "" {
		dbPath = "/Root/testdb"
	}

	// Initialize blobstorage.
	e.emitLine("initializing YDB blobstorage...")
	initCmd := fmt.Sprintf(
		`LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd -s %s admin blobstorage config init --yaml-file %s`,
		endpoint, confPath)
	if _, err := e.shell(ctx, initCmd); err != nil {
		return fmt.Errorf("blobstorage init: %w", err)
	}

	// Create database.
	e.emitLine(fmt.Sprintf("creating YDB database %s...", dbPath))
	createCmd := fmt.Sprintf(
		`LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd -s %s admin database %s create ssd:1`,
		endpoint, dbPath)
	if _, err := e.shell(ctx, createCmd); err != nil {
		return fmt.Errorf("create database: %w", err)
	}

	e.emitLine("YDB cluster initialized")
	return nil
}
```

- [ ] **Step 5: Implement startYDBDB (dynamic node)**

```go
// startYDBDB starts a YDB dynamic (database) node.
func (e *Executor) startYDBDB(ctx context.Context, cmd Command) error {
	var cfg YDBDatabaseConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	dbPath := cfg.DatabasePath
	if dbPath == "" {
		dbPath = "/Root/testdb"
	}

	// Build --node-broker flags.
	var brokerFlags strings.Builder
	for _, ep := range cfg.StaticEndpoints {
		fmt.Fprintf(&brokerFlags, " --node-broker grpc://%s:2136", ep)
	}

	// Start dynamic node via systemd-run.
	e.shell(ctx, "systemctl stop ydbd-database 2>/dev/null; systemctl reset-failed ydbd-database 2>/dev/null")
	e.emitLine("starting YDB dynamic (database) node...")
	startCmd := fmt.Sprintf(
		`systemd-run --unit=ydbd-database --uid=ydb --gid=ydb `+
			`--setenv=LD_LIBRARY_PATH=/opt/ydb/lib `+
			`/opt/ydb/bin/ydbd server `+
			`--yaml-config /opt/ydb/cfg/config.yaml `+
			`--grpc-port 2136 --ic-port 19002 --mon-port 8766 `+
			`--tenant %s%s`,
		dbPath, brokerFlags.String())
	if _, err := e.shell(ctx, startCmd); err != nil {
		return fmt.Errorf("start ydbd-database: %w", err)
	}

	// Wait for gRPC port.
	if _, err := e.shell(ctx, `for i in $(seq 1 30); do nc -z localhost 2136 && exit 0; sleep 1; done; exit 1`); err != nil {
		return fmt.Errorf("ydbd-database did not start: %w", err)
	}

	e.emitLine("YDB database node started")
	return nil
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./...`

---

### Task 5: DAG Tasks

**Files:**
- Create: `internal/domain/run/task_ydb.go`

- [ ] **Step 1: Create task_ydb.go**

```go
package run

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// ydbInstallTask installs ydbd binary on all DB targets.
type ydbInstallTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.YDBTopology
	pkg      *types.Package
}

func (t *ydbInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing YDB on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallYDB,
		Config: agent.YDBInstallConfig{Version: t.version},
	})
}

// ydbConfigTask starts static (storage) nodes on all DB targets.
type ydbConfigTask struct {
	client   agent.Client
	state    *State
	topology *types.YDBTopology
}

func (t *ydbConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring YDB static nodes")

	// For split topology, only storage-role targets get static config.
	// For combined mode, all targets are both storage and database.
	// Currently all DB targets are storage nodes; dynamic-only targets
	// would require a separate machine role (future enhancement).

	hosts := make([]string, len(targets))
	for i, tgt := range targets {
		h := tgt.InternalHost
		if h == "" {
			h = tgt.Host
		}
		hosts[i] = h
	}

	diskPath := "/ydb_data"
	ft := t.topology.FaultTolerance
	if ft == "" {
		ft = "none"
	}

	for i, target := range targets {
		advHost := target.InternalHost
		if advHost == "" {
			advHost = target.Host
		}
		cfg := agent.YDBStaticConfig{
			Hosts:          hosts,
			InstanceID:     i,
			AdvertiseHost:  advHost,
			DiskPath:       diskPath,
			FaultTolerance: ft,
			Options:        t.topology.StorageOptions,
		}
		if err := t.client.Send(nc, target, agent.Command{
			Action: agent.ActionConfigYDB, Config: cfg,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ydbInitTask initializes the YDB cluster (blobstorage + create database).
// Runs on the FIRST static node only.
type ydbInitTask struct {
	client   agent.Client
	state    *State
	topology *types.YDBTopology
}

func (t *ydbInitTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	if len(targets) == 0 {
		return fmt.Errorf("no DB targets for YDB init")
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

	nc.Log().Info("initializing YDB cluster", "host", host)
	return t.client.Send(nc, first, agent.Command{
		Action: agent.ActionInitYDB,
		Config: agent.YDBInitConfig{
			StaticEndpoint: fmt.Sprintf("grpc://%s:2136", host),
			DatabasePath:   dbPath,
			ConfigPath:     "/opt/ydb/cfg/config.yaml",
		},
	})
}

// ydbStartDBTask starts dynamic (database) nodes.
// In combined mode, this runs on the same targets as storage.
type ydbStartDBTask struct {
	client   agent.Client
	state    *State
	topology *types.YDBTopology
}

func (t *ydbStartDBTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("starting YDB database nodes")

	// Static endpoints for node-broker discovery.
	staticHosts := make([]string, len(targets))
	for i, tgt := range targets {
		h := tgt.InternalHost
		if h == "" {
			h = tgt.Host
		}
		staticHosts[i] = h
	}

	dbPath := t.topology.DatabasePath
	if dbPath == "" {
		dbPath = "/Root/testdb"
	}

	// In combined mode, start dynamic on same targets.
	// In split mode, dynamic targets would be separate (future: separate role).
	for _, target := range targets {
		advHost := target.InternalHost
		if advHost == "" {
			advHost = target.Host
		}
		cfg := agent.YDBDatabaseConfig{
			StaticEndpoints: staticHosts,
			AdvertiseHost:   advHost,
			DatabasePath:    dbPath,
			Options:         t.topology.DatabaseOptions,
		}
		if err := t.client.Send(nc, target, agent.Command{
			Action: agent.ActionStartYDBDB, Config: cfg,
		}); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

---

### Task 6: DAG Builder Wiring

**Files:**
- Modify: `internal/domain/run/builder.go`

- [ ] **Step 1: Add YDB case to dbTasks()**

Add case in the `dbTasks()` switch:

```go
	case types.DatabaseYDB:
		return &ydbInstallTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.YDB, pkg: pkg},
			&ydbConfigTask{client: b.deps.Client, state: b.deps.State, topology: db.YDB}, nil
```

- [ ] **Step 2: Add needsYDBInit helper**

```go
func (b *builder) needsYDBInit() bool {
	return b.cfg.Database.Kind == types.DatabaseYDB && b.cfg.Database.YDB != nil
}
```

- [ ] **Step 3: Add addYDBPhases helper**

```go
func (b *builder) addYDBPhases() {
	// init_ydb_cluster depends on configure_db (all static nodes running).
	b.add(b.ph(types.PhaseInitYDBCluster), []string{b.ph(types.PhaseConfigureDB)},
		&ydbInitTask{client: b.deps.Client, state: b.deps.State, topology: b.cfg.Database.YDB})

	// start_ydb_database depends on init_ydb_cluster.
	b.add(b.ph(types.PhaseStartYDBDatabase), []string{b.ph(types.PhaseInitYDBCluster)},
		&ydbStartDBTask{client: b.deps.Client, state: b.deps.State, topology: b.cfg.Database.YDB})

	// run_stroppy must wait for database nodes, not just storage.
	b.runStroppyDeps = append(b.runStroppyDeps, b.ph(types.PhaseStartYDBDatabase))
}
```

- [ ] **Step 4: Wire into build() method**

In `build()`, after the proxy section and before the stroppy section, add:

```go
	// --- YDB cluster init + database start ---
	if b.needsYDBInit() {
		b.addYDBPhases()
	}
```

Also update the `configure_monitor` dependency: when YDB, it should depend on `start_ydb_database` instead of `configure_db`. Update the monitoring section:

```go
	// Configure/start daemons after install AND after DB is fully ready.
	monitorDeps := []string{b.ph(types.PhaseInstallMonitor), b.ph(types.PhaseConfigureDB)}
	if b.needsYDBInit() {
		monitorDeps = append(monitorDeps, b.ph(types.PhaseStartYDBDatabase))
	}
	b.add(b.ph(types.PhaseConfigureMonitor), monitorDeps,
		&monitorConfigTask{...})
```

- [ ] **Step 5: Update needsProxy for YDB**

Add case to `needsProxy()`:

```go
	case types.DatabaseYDB:
		return db.YDB != nil && db.YDB.HAProxy != nil
```

- [ ] **Step 6: Verify build**

Run: `go build ./...`

---

### Task 7: Infrastructure — Ports, Endpoints, Proxy, Stroppy Driver

**Files:**
- Modify: `internal/domain/run/task_infra.go`
- Modify: `internal/domain/run/task_stroppy.go`
- Modify: `internal/domain/run/task_proxy.go`

- [ ] **Step 1: Add YDB port in task_infra.go dockerMachines**

Add case in the port switch (around line 167):

```go
				case types.DatabaseYDB:
					dbPort = 2136
```

- [ ] **Step 2: Add YDB port in task_infra.go yandexMachines**

Add case in the YC port switch (around line 442):

```go
				case types.DatabaseYDB:
					dbPort = 2136
```

- [ ] **Step 3: Add YDB proxy routing in both docker and yandex paths**

In the proxy routing section (docker path, ~line 197):

```go
			case types.DatabaseYDB:
				t.state.SetDBEndpoint(proxyHost, 2136) // HAProxy gRPC passthrough
```

Same in the yandex path proxy routing section.

- [ ] **Step 4: Add YDB driver URL in task_stroppy.go**

Add case in the driver URL switch:

```go
	case types.DatabaseYDB:
		dbPath := "Root/testdb"
		if cfg.Database.YDB != nil && cfg.Database.YDB.DatabasePath != "" {
			dbPath = strings.TrimPrefix(cfg.Database.YDB.DatabasePath, "/")
		}
		driverURL = fmt.Sprintf("grpc://%s:%d/%s", dbHost, dbPort, dbPath)
		driverType = "ydb"
```

Note: `task_stroppy.go` needs access to `cfg.Database.YDB` — check if `stroppyRunTask` already has the db config, otherwise add `dbKind` field usage or pass topology.

- [ ] **Step 5: Add YDB to proxy install/config**

In `task_proxy.go` `proxyInstallTask.Execute()`, add YDB to the HAProxy case:

```go
	case types.DatabasePostgres, types.DatabasePicodata, types.DatabaseYDB:
```

Add `configHAProxyYDB` method:

```go
func (t *proxyConfigTask) configHAProxyYDB(nc *dag.NodeContext, proxyTargets, dbTargets []agent.Target) error {
	nc.Log().Info("configuring haproxy for YDB (gRPC)")

	var backends []string
	for _, tgt := range dbTargets {
		host := tgt.InternalHost
		if host == "" {
			host = tgt.Host
		}
		backends = append(backends, fmt.Sprintf("%s:2136", host))
	}

	cfg := agent.HAProxyConfig{
		DBKind:      "ydb",
		WritePort:   2136,
		ReadPort:    2137,
		Backends:    backends,
		HealthCheck: "tcp",
	}

	return t.client.SendAll(nc, proxyTargets, agent.Command{
		Action: agent.ActionConfigHAProxy,
		Config: cfg,
	})
}
```

Add case in `proxyConfigTask.Execute()`:

```go
	case types.DatabaseYDB:
		return t.configHAProxyYDB(nc, targets, dbTargets)
```

Add `ydbTopology *types.YDBTopology` field to `proxyConfigTask` struct and pass it from `builder.go` `addProxy()`.

- [ ] **Step 6: Verify build**

Run: `go build ./...`

---

### Task 8: Validation and Machine Extraction

**Files:**
- Modify: `internal/domain/run/validate.go`
- Modify: `internal/domain/run/machines.go`

- [ ] **Step 1: Add YDB to script compatibility**

In `validate.go`, update `scriptDBSupport`:

```go
var scriptDBSupport = map[string][]types.DatabaseKind{
	"tpcc/procs": {types.DatabasePostgres, types.DatabaseMySQL},
	"tpcc/tx":    {types.DatabasePostgres, types.DatabaseMySQL, types.DatabasePicodata, types.DatabaseYDB},
	"tpcb/procs": {types.DatabasePostgres, types.DatabaseMySQL},
	"tpcb/tx":    {types.DatabasePostgres, types.DatabaseMySQL, types.DatabasePicodata, types.DatabaseYDB},
}
```

- [ ] **Step 2: Add YDB topology validation**

Add case in the topology-matching switch:

```go
	case types.DatabaseYDB:
		if cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.Picodata != nil {
			return fmt.Errorf("database.kind is ydb but non-ydb topology is set")
		}
```

Also update the "at least one topology" check:

```go
	if cfg.Database.Postgres == nil && cfg.Database.MySQL == nil && cfg.Database.Picodata == nil && cfg.Database.YDB == nil && cfg.PresetID == "" {
```

- [ ] **Step 3: Add YDB machine extraction**

In `machines.go`, add case to `FillMachinesFromTopology`:

```go
	case types.DatabaseYDB:
		if db.YDB != nil {
			cfg.Machines = append(cfg.Machines, types.MachineSpec{
				Role: types.RoleDatabase, Count: db.YDB.Storage.Count,
				CPUs: db.YDB.Storage.CPUs, MemoryMB: db.YDB.Storage.MemoryMB, DiskGB: db.YDB.Storage.DiskGB,
			})
			if db.YDB.Database != nil {
				cfg.Machines = append(cfg.Machines, types.MachineSpec{
					Role: types.RoleDatabase, Count: db.YDB.Database.Count,
					CPUs: db.YDB.Database.CPUs, MemoryMB: db.YDB.Database.MemoryMB, DiskGB: db.YDB.Database.DiskGB,
				})
			}
			if db.YDB.HAProxy != nil {
				cfg.Machines = append(cfg.Machines, *db.YDB.HAProxy)
			}
		}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`

---

### Task 9: Monitoring — Metrics and vmagent Config

**Files:**
- Modify: `internal/domain/metrics/queries.go`
- Modify: `internal/domain/agent/executor.go` (configMonitor section)
- Modify: `internal/domain/api/server.go`

- [ ] **Step 1: Add ydbMetrics()**

In `queries.go`, add:

```go
func ydbMetrics() []MetricDef {
	return []MetricDef{
		{
			Name:  "DB gRPC Requests/s",
			Key:   "db_qps",
			Query: `sum(rate(api_grpc_request_count_total{%s}[5m]))`,
			Unit:  "req/s",
		},
		{
			Name:  "DB gRPC Errors/s",
			Key:   "db_errors",
			Query: `sum(rate(api_grpc_request_errors_total{%s}[5m]))`,
			Unit:  "err/s",
		},
		{
			Name:  "DB Active Sessions",
			Key:   "db_sessions",
			Query: `sum(table_service_active_sessions{%s})`,
			Unit:  "",
		},
		{
			Name:  "DB Tablet Count",
			Key:   "db_tablets",
			Query: `sum(ydb_tablets_count{%s})`,
			Unit:  "",
		},
	}
}
```

- [ ] **Step 2: Add YDB case to MetricsForDB**

```go
	case "ydb":
		dbMetrics = ydbMetrics()
```

- [ ] **Step 3: Add YDB scrape job to configMonitor in executor.go**

In the `configMonitor` handler, find where per-DB scrape jobs are added (mysqld_exporter, picodata sections). Add a YDB section:

```go
	case "ydb":
		// YDB exports Prometheus metrics natively on :8765 (static) and :8766 (dynamic).
		for _, tgt := range dbTargets {
			host := tgt.InternalHost
			if host == "" {
				host = tgt.Host
			}
			// Static node metrics.
			scrapeConfigs = append(scrapeConfigs, fmt.Sprintf(`
  - job_name: ydb_%s
    metrics_path: /counters/counters=ydb/prometheus
    static_configs:
    - targets: ['%s:8765']
      labels:
        stroppy_machine_id: '%s'`, tgt.ID, host, tgt.ID))
		}
```

- [ ] **Step 4: Add Grafana dashboard reference**

In `server.go`, add to `grafanaDashboards` map:

```go
			"ydb": "stroppy-ydb",
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`

---

### Task 10: Frontend — Types, NewRun, LogStream

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/pages/NewRun.tsx`
- Modify: `web/src/components/LogStream.tsx`

- [ ] **Step 1: Add YDB types**

In `types.ts`, update `DatabaseKind`:

```typescript
export type DatabaseKind = "postgres" | "mysql" | "picodata" | "ydb";
```

Add `YDBTopology` interface:

```typescript
export interface YDBTopology {
  storage: MachineSpec;
  database?: MachineSpec;
  haproxy?: MachineSpec;
  fault_tolerance: string;
  database_path: string;
  storage_options?: Record<string, string>;
  database_options?: Record<string, string>;
  haproxy_options?: Record<string, string>;
}
```

Add `ydb?: YDBTopology` to `DatabaseConfig` interface.

- [ ] **Step 2: Update NewRun.tsx**

Add YDB to `DB_KINDS`:

```typescript
const DB_KINDS: DatabaseKind[] = ["postgres", "mysql", "picodata", "ydb"];
```

Add to `DB_VERSIONS`:

```typescript
  ydb: ["25.3"],
```

Add to `DB_META`:

```typescript
  ydb: { icon: Database, label: "YDB" },
```

- [ ] **Step 3: Update LogStream action labels**

In `LogStream.tsx`, add to `ACTION_LABELS`:

```typescript
  install_ydb: "Install YDB",
  config_ydb: "Configure YDB Static",
  init_ydb: "Init YDB Cluster",
  start_ydb_db: "Start YDB Database",
  init_ydb_cluster: "Init YDB Cluster",
  start_ydb_database: "Start YDB Database",
```

Add to `PHASE_ACTIONS`:

```typescript
  init_ydb_cluster: ["init_ydb"],
  start_ydb_database: ["start_ydb_db"],
```

Update `install_db` and `configure_db` arrays:

```typescript
  install_db: ["install_postgres", "install_mysql", "install_picodata", "install_ydb"],
  configure_db: ["config_postgres", "config_mysql", "config_picodata", "config_ydb"],
```

- [ ] **Step 4: Verify frontend builds**

Run: `cd web && npx tsc --noEmit`

---

### Task 11: PresetDesigner YDB Topology Editor

**Files:**
- Modify: `web/src/pages/PresetDesigner.tsx`

- [ ] **Step 1: Add YDB topology form section**

Add a YDB section to the PresetDesigner that renders when `dbKind === "ydb"`. It should include:
- Storage node spec (CPU/RAM/Disk sliders + count)
- Database node spec (optional toggle for split mode, CPU/RAM/Disk sliders + count)
- HAProxy toggle + spec
- Fault tolerance selector (none / block-4-2 / mirror-3-dc)
- Database path input (default `/Root/testdb`)
- Storage options editor
- Database options editor

Follow the existing pattern for Picodata topology (instances + haproxy + options).

- [ ] **Step 2: Add YDB validation**

```typescript
function validateYDB(t: YDBTopology): string[] {
  const errors: string[] = [];
  if (t.storage.count < 1) errors.push("At least 1 storage node required");
  if (t.database && t.database.count < 1) errors.push("Database node count must be >= 1");
  if (t.storage.cpus < 1) errors.push("Storage CPUs must be >= 1");
  if (t.storage.memory_mb < 2048) errors.push("Storage RAM must be >= 2 GB");
  if (t.storage.disk_gb < 80) errors.push("Storage disk must be >= 80 GB");
  return errors;
}
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd web && npx tsc --noEmit`

---

### Task 12: Integration Test — Docker Run

- [ ] **Step 1: Build and start server**

```bash
docker compose build --no-cache server
docker compose up -d server
```

- [ ] **Step 2: Verify presets seeded**

```bash
TOKEN=$(curl -s http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')

curl -s http://localhost:8080/api/v1/presets -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
for p in json.load(sys.stdin):
    if 'YDB' in p['name']: print(f'{p[\"name\"]}: {p[\"db_kind\"]}')"
```

Expected: YDB single, YDB cluster, YDB scale presets visible.

- [ ] **Step 3: Start YDB single run**

```bash
curl -s http://localhost:8080/api/v1/run \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "provider": "docker",
    "network": {"cidr": "10.0.0.0/8"},
    "database": {"kind": "ydb", "version": "25.3", "ydb": {
      "storage": {"role":"database","count":1,"cpus":2,"memory_mb":4096,"disk_gb":80},
      "fault_tolerance": "none",
      "database_path": "/Root/testdb"
    }},
    "stroppy": {"script": "tpcb/tx", "duration": "30s", "vus": 1},
    "monitor": {}
  }'
```

- [ ] **Step 4: Monitor run progress**

Watch server logs for all YDB phases:
```bash
docker compose logs -f server | grep -E "ydb|YDB|init_ydb|start_ydb"
```

Expected phases in order: install_db → configure_db → init_ydb_cluster → start_ydb_database → run_stroppy → teardown

- [ ] **Step 5: Verify metrics collection**

Check that YDB Prometheus metrics are being scraped via vmagent.

- [ ] **Step 6: Fix issues found during testing**

Address any errors from the test run — YDB config format, port conflicts, timing issues.
