package dagbuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// CatalogPort is the subset of catalog the builder needs. Defined locally
// to avoid an import cycle with the parent testing package.
type CatalogPort interface {
	GetDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId) (*catalogpb.DatabasePreset, error)
	GetWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId) (*catalogpb.WorkloadPreset, error)
	GetPackage(ctx context.Context, id *catalogpb.PackageId) (*catalogpb.Package, error)
	ListPackages(ctx context.Context, tenantID *iampb.TenantId, dbKind *catalogpb.Database_Kind) ([]*catalogpb.Package, error)
}

// Builder converts a TestRun into a system.Dag template.
type Builder struct{ catalog CatalogPort }

// New wires the builder against a CatalogPort.
func New(catalog CatalogPort) *Builder { return &Builder{catalog: catalog} }

// machine is the inventory entry stored in Dag.Metadata["machines"] and
// passed into the terraform task Vars under the same key.
type machine struct {
	ID   string
	Role catalogpb.MachineRole
	Kind string // engine kind name when the machine hosts the DB process; empty otherwise
}

func (m machine) roleString() string { return m.Role.String() }

// FromTestRun expands a TestRun into a full per-engine DAG with concrete
// per-machine targeting. Shape (self-hosted engines):
//
//	provision               (Terraform OP_APPLY)        — issues bootstrap tokens
//	wait-agents             (WaitAgentsTask)            — block until every machine_id binds
//	install-db              (PackageInstall, DB nodes)
//	install-monitor         (PackageInstall, monitor-0)
//	config-db               (ConfigApply, DB nodes)
//	init-db                 (OneShot, single primary)   — only when engine needs init
//	render-stroppy-config   (ConfigApply, stroppy-0)    — writes /tmp/stroppy.cfg.json
//	stroppy-run             (StroppyRun, stroppy-0)
//	teardown                (Terraform OP_DESTROY)
//
// External DB short-circuit: only render-stroppy-config + stroppy-run.
// Managed YDB: provision + render-stroppy-config + stroppy-run + teardown.
func (b *Builder) FromTestRun(ctx context.Context, tr *testingpb.TestRun) (*systempb.Dag, error) {
	db, err := b.resolveDatabase(ctx, tr.GetDatabase())
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: resolve database: %w", err)
	}
	wl, err := b.resolveWorkload(ctx, tr.GetWorkload())
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: resolve workload: %w", err)
	}

	if !supported(db.GetKind()) {
		return nil, fmt.Errorf("dagbuilder: unsupported database kind %s — add a branch in planForEngine before using this engine", db.GetKind().String())
	}
	plan := planForEngine(db.GetKind())
	if db.GetExternal() != nil {
		plan.external = true
	}

	// External: stroppy-only path.
	if plan.external {
		stroppyOnly, err := b.externalDag(wl)
		if err != nil {
			return nil, err
		}
		return stroppyOnly, nil
	}

	machines := planMachines(db, plan)
	if len(machines) == 0 {
		return nil, fmt.Errorf("dagbuilder: no machines planned for kind=%s", db.GetKind())
	}

	tenantID := tr.GetTenantId().GetValue()
	machinesStructList := machineInventoryStruct(machines)

	// vars passed to terraform; the handler injects agent_bootstrap_tokens
	// before apply.
	tfVars, err := structpb.NewStruct(map[string]any{
		"tenant_id": tenantID,
		"machines":  machinesStructList,
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: tfvars: %w", err)
	}

	var nodes []*systempb.Dag_Node

	provision, err := mkNode("provision", nil, &taskspb.TerraformTask{
		Op:              taskspb.TerraformTask_OP_APPLY,
		Module:          plan.module,
		Vars:            tfVars,
		WdIdStateKey:    "yc.wd",
		OutputStateKeys: map[string]string{"db_url": "db.url"},
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: provision: %w", err)
	}
	nodes = append(nodes, provision)

	allIDs := machineIDs(machines)
	waitAgents, err := mkNode("wait-agents", []string{"provision"}, &taskspb.WaitAgentsTask{
		MachineIds:     allIDs,
		TimeoutSeconds: 600,
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: wait-agents: %w", err)
	}
	nodes = append(nodes, waitAgents)

	prev := []string{"wait-agents"}
	stroppyDeps := prev

	dbIDs := filterIDs(machines, isDatabaseRole)

	if plan.installDB && len(dbIDs) > 0 {
		// Resolve the package recipe so the agent gets the install payload inline.
		// For managed-YDB and external this branch is skipped.
		dbPkg, err := b.resolvePackageForDB(ctx, tenantID, db.GetKind())
		if err != nil {
			return nil, fmt.Errorf("dagbuilder: resolve db package: %w", err)
		}
		monPkg, err := b.resolveMonitorPackage(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("dagbuilder: resolve monitor package: %w", err)
		}

		installDB, err := mkNode("install-db", prev, installTaskFromPackage(dbPkg, dbIDs, catalogpb.MachineRole_MACHINE_ROLE_DATABASE))
		if err != nil {
			return nil, fmt.Errorf("dagbuilder: install-db: %w", err)
		}
		nodes = append(nodes, installDB)

		monIDs := filterIDs(machines, isMonitorRole)
		if len(monIDs) > 0 {
			installMon, err := mkNode("install-monitor", prev, installTaskFromPackage(monPkg, monIDs, catalogpb.MachineRole_MACHINE_ROLE_MONITOR))
			if err != nil {
				return nil, fmt.Errorf("dagbuilder: install-monitor: %w", err)
			}
			nodes = append(nodes, installMon)
		}

		configDB, err := mkNode("config-db", []string{"install-db"}, &taskspb.ConfigApplyTask{
			EngineKind:       db.GetKind(),
			Role:             catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
			TargetMachineIds: dbIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("dagbuilder: config-db: %w", err)
		}
		nodes = append(nodes, configDB)
		stroppyDeps = []string{"config-db"}

		if plan.initCmd != "" {
			initDB, err := mkNode("init-db", []string{"config-db"}, &taskspb.OneShotTask{
				EngineKind:       db.GetKind(),
				Role:             catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
				Target:           taskspb.OneShotTask_TARGET_FIRST,
				TargetMachineIds: []string{dbIDs[0]},
				CommandTemplate:  plan.initCmd,
			})
			if err != nil {
				return nil, fmt.Errorf("dagbuilder: init-db: %w", err)
			}
			nodes = append(nodes, initDB)
			stroppyDeps = []string{"init-db"}
		}
	}

	// render-stroppy-config: a ConfigApply targeted at the stroppy worker.
	// We pre-render the config-json client-side using the resolved workload
	// + a state-store $DB_URL reference. Since the agent merely writes
	// bytes, the render happens here.
	stroppyConfigPath := "/tmp/stroppy.cfg.json"
	stroppyCfgBytes, err := renderStroppyConfig(db, wl)
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: render stroppy config: %w", err)
	}
	renderCfg, err := mkNode("render-stroppy-config", stroppyDeps, &taskspb.ConfigApplyTask{
		EngineKind:       db.GetKind(),
		Role:             catalogpb.MachineRole_MACHINE_ROLE_STROPPY,
		TargetMachineIds: []string{"stroppy-0"},
		Files: []*commonpb.ConfigFile{
			{Path: stroppyConfigPath, Inline: string(stroppyCfgBytes), Mode: 0644},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: render-stroppy-config: %w", err)
	}
	nodes = append(nodes, renderCfg)

	// render-stroppy-config-url: substitute the "__DB_URL__" sentinel in the
	// stroppy config with the runtime DB URL written to the state-store by
	// the terraform apply handler under "db.url".
	renderCfgURL, err := mkNode("render-stroppy-config-url", []string{"render-stroppy-config"}, &taskspb.OneShotTask{
		EngineKind:       db.GetKind(),
		Role:             catalogpb.MachineRole_MACHINE_ROLE_STROPPY,
		Target:           taskspb.OneShotTask_TARGET_FIRST,
		TargetMachineIds: []string{"stroppy-0"},
		CommandTemplate:  `sed -i "s|__DB_URL__|{{.DB_URL}}|" ` + stroppyConfigPath,
		StateVarRefs:     map[string]string{"DB_URL": "db.url"},
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: render-stroppy-config-url: %w", err)
	}
	nodes = append(nodes, renderCfgURL)

	stroppyRun, err := mkNode("stroppy-run", []string{"render-stroppy-config-url"}, &taskspb.StroppyRunTask{
		Workload:           wl,
		Version:            "",
		DbUrlStateKey:      "db.url",
		TargetMachineIds:   []string{"stroppy-0"},
		ConfigPathStateKey: "stroppy.config_path",
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: stroppy-run: %w", err)
	}
	nodes = append(nodes, stroppyRun)

	teardown, err := mkNode("teardown", []string{"stroppy-run"}, &taskspb.TerraformTask{
		Op:           taskspb.TerraformTask_OP_DESTROY,
		Module:       plan.module,
		WdIdStateKey: "yc.wd",
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: teardown: %w", err)
	}
	nodes = append(nodes, teardown)

	dagMeta, _ := structpb.NewStruct(map[string]any{
		"machines": machinesStructList,
	})

	return &systempb.Dag{
		Graph:    &systempb.Dag_Graph{Nodes: nodes},
		Metadata: dagMeta,
	}, nil
}

// externalDag returns the stroppy-only DAG for the BYOD short-circuit.
// Even external runs need a stroppy worker — but since we don't provision
// infra for external, we only emit a stroppy-run node and assume the
// workload binary is already present at the caller's machine (the smoke
// path triggers this from an outside runner).
func (b *Builder) externalDag(wl *catalogpb.Workload) (*systempb.Dag, error) {
	stroppyRun, err := mkNode("stroppy-run", nil, &taskspb.StroppyRunTask{
		Workload:           wl,
		DbUrlStateKey:      "db.url",
		TargetMachineIds:   []string{"stroppy-0"},
		ConfigPathStateKey: "stroppy.config_path",
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: stroppy-run (external): %w", err)
	}
	return &systempb.Dag{Graph: &systempb.Dag_Graph{Nodes: []*systempb.Dag_Node{stroppyRun}}}, nil
}

// planMachines materialises a concrete inventory of machines for the
// resolved database shape. Always includes monitor-0 + stroppy-0; managed
// engines collapse to those plus zero DB-role machines (the database lives
// in YC).
func planMachines(db *catalogpb.Database, plan enginePlan) []machine {
	var ms []machine
	kindName := db.GetKind().String()

	switch db.GetKind() {
	case catalogpb.Database_DATABASE_KIND_POSTGRES:
		shape := db.GetPostgres().GetShape()
		ms = append(ms, machine{ID: "pg-master-0", Role: catalogpb.MachineRole_MACHINE_ROLE_DATABASE, Kind: kindName})
		for i := uint32(0); i < shape.GetReplicas(); i++ {
			ms = append(ms, machine{ID: fmt.Sprintf("pg-replica-%d", i), Role: catalogpb.MachineRole_MACHINE_ROLE_DATABASE, Kind: kindName})
		}
		if shape.GetPatroni() && !shape.GetEtcdColocated() {
			// Default 3 etcd nodes for HA when not colocated.
			for i := 0; i < 3; i++ {
				ms = append(ms, machine{ID: fmt.Sprintf("etcd-%d", i), Role: catalogpb.MachineRole_MACHINE_ROLE_ETCD})
			}
		}
		if shape.GetPgbouncerColocated() {
			// colocated → no separate row
		} else if shape.GetHaproxyDedicated() {
			ms = append(ms, machine{ID: "haproxy-0", Role: catalogpb.MachineRole_MACHINE_ROLE_PROXY})
		}
	case catalogpb.Database_DATABASE_KIND_MYSQL, catalogpb.Database_DATABASE_KIND_MARIADB:
		shape := db.GetMysql().GetShape()
		ms = append(ms, machine{ID: "mysql-master-0", Role: catalogpb.MachineRole_MACHINE_ROLE_DATABASE, Kind: kindName})
		for i := uint32(0); i < shape.GetReplicas(); i++ {
			ms = append(ms, machine{ID: fmt.Sprintf("mysql-replica-%d", i), Role: catalogpb.MachineRole_MACHINE_ROLE_DATABASE, Kind: kindName})
		}
		if shape.GetProxysqlDedicated() {
			ms = append(ms, machine{ID: "proxysql-0", Role: catalogpb.MachineRole_MACHINE_ROLE_PROXY})
		}
	case catalogpb.Database_DATABASE_KIND_PICODATA:
		shape := db.GetPicodata().GetShape()
		count := shape.GetNodes()
		if count == 0 {
			count = 1
		}
		for i := uint32(0); i < count; i++ {
			ms = append(ms, machine{ID: fmt.Sprintf("picodata-%d", i), Role: catalogpb.MachineRole_MACHINE_ROLE_DATABASE, Kind: kindName})
		}
		if shape.GetHaproxyDedicated() {
			ms = append(ms, machine{ID: "haproxy-0", Role: catalogpb.MachineRole_MACHINE_ROLE_PROXY})
		}
	case catalogpb.Database_DATABASE_KIND_COCKROACH:
		shape := db.GetCockroach().GetShape()
		count := shape.GetNodes()
		if count == 0 {
			count = 1
		}
		for i := uint32(0); i < count; i++ {
			ms = append(ms, machine{ID: fmt.Sprintf("cockroach-%d", i), Role: catalogpb.MachineRole_MACHINE_ROLE_DATABASE, Kind: kindName})
		}
	case catalogpb.Database_DATABASE_KIND_YDB:
		shape := db.GetYdb().GetShape()
		storage := shape.GetStorageNodes()
		if storage == 0 {
			storage = 1
		}
		for i := uint32(0); i < storage; i++ {
			ms = append(ms, machine{ID: fmt.Sprintf("ydb-storage-%d", i), Role: catalogpb.MachineRole_MACHINE_ROLE_YDB_STORAGE, Kind: kindName})
		}
		for i := uint32(0); i < shape.GetDatabaseNodes(); i++ {
			ms = append(ms, machine{ID: fmt.Sprintf("ydb-database-%d", i), Role: catalogpb.MachineRole_MACHINE_ROLE_YDB_DATABASE, Kind: kindName})
		}
	case catalogpb.Database_DATABASE_KIND_YDB_MANAGED:
		// Managed YDB: no on-VM DB process. Only a stroppy worker + monitor
		// (added below). Provision allocates the managed YDB cluster itself.
	}

	// Always include monitoring and the stroppy worker.
	ms = append(ms, machine{ID: "monitor-0", Role: catalogpb.MachineRole_MACHINE_ROLE_MONITOR})
	ms = append(ms, machine{ID: "stroppy-0", Role: catalogpb.MachineRole_MACHINE_ROLE_STROPPY})

	_ = plan
	return ms
}

func isDatabaseRole(m machine) bool {
	switch m.Role {
	case catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
		catalogpb.MachineRole_MACHINE_ROLE_YDB_STORAGE,
		catalogpb.MachineRole_MACHINE_ROLE_YDB_DATABASE:
		return true
	}
	return false
}

func isMonitorRole(m machine) bool { return m.Role == catalogpb.MachineRole_MACHINE_ROLE_MONITOR }

func machineIDs(ms []machine) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

func filterIDs(ms []machine, pred func(machine) bool) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		if pred(m) {
			out = append(out, m.ID)
		}
	}
	return out
}

func machineInventoryStruct(ms []machine) []any {
	out := make([]any, 0, len(ms))
	for _, m := range ms {
		out = append(out, map[string]any{
			"id":   m.ID,
			"role": m.roleString(),
			"kind": m.Kind,
		})
	}
	return out
}

// resolvePackageForDB picks the first non-deleted Package row visible to
// the tenant with the requested engine kind. Returns an error when no row
// matches — operator must seed a builtin or create a tenant-scoped package
// before launching.
func (b *Builder) resolvePackageForDB(ctx context.Context, tenantID string, kind catalogpb.Database_Kind) (*catalogpb.Package, error) {
	list, err := b.catalog.ListPackages(ctx, &iampb.TenantId{Value: tenantID}, &kind)
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		if p == nil {
			continue
		}
		if p.GetTimestamps() != nil && p.GetTimestamps().GetDeletedAt() != nil {
			continue
		}
		return p, nil
	}
	return nil, fmt.Errorf("dagbuilder: no Package row found for tenant=%s kind=%s; seed a builtin or create a tenant package", tenantID, kind.String())
}

// resolveMonitorPackage picks the first non-deleted Package whose name
// starts with "Monitor" (kind-agnostic). Returns an error when no row
// matches.
func (b *Builder) resolveMonitorPackage(ctx context.Context, tenantID string) (*catalogpb.Package, error) {
	list, err := b.catalog.ListPackages(ctx, &iampb.TenantId{Value: tenantID}, nil)
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		if p == nil {
			continue
		}
		if p.GetTimestamps() != nil && p.GetTimestamps().GetDeletedAt() != nil {
			continue
		}
		if strings.HasPrefix(p.GetIdentity().GetName(), "Monitor") {
			return p, nil
		}
	}
	return nil, fmt.Errorf("dagbuilder: no Monitor Package row found for tenant=%s; seed builtins or create a Monitor package", tenantID)
}

// installTaskFromPackage materialises a PackageInstallTask with the
// Package recipe inlined. nil pkg → empty inlined fields (handler is a
// no-op once it sees no apt_packages / pre_install / deb_blob_url).
func installTaskFromPackage(pkg *catalogpb.Package, targets []string, role catalogpb.MachineRole) *taskspb.PackageInstallTask {
	task := &taskspb.PackageInstallTask{
		Role:             role,
		TargetMachineIds: targets,
	}
	if pkg == nil {
		return task
	}
	if pkg.GetId() != nil {
		task.PackageId = pkg.GetId()
	}
	switch src := pkg.GetSource().GetSource().(type) {
	case *catalogpb.Package_PackageSource_Apt:
		task.AptPackages = src.Apt.GetAptPackages()
		task.PreInstall = src.Apt.GetPreInstall()
		task.CustomRepo = src.Apt.GetCustomRepo()
		task.CustomRepoKey = src.Apt.GetCustomRepoKey()
	case *catalogpb.Package_PackageSource_DebBlob:
		task.PreInstall = src.DebBlob.GetPreInstall()
		task.DebBlobUrl = fmt.Sprintf("/packages/%s/deb?filename=%s",
			pkg.GetId().GetValue(), src.DebBlob.GetDebFilename())
		task.DebBlobSha256 = src.DebBlob.GetSha256()
	}
	return task
}

// renderStroppyConfig serialises the workload + DB topology into the JSON
// blob the stroppy binary expects on disk. Mirrors stroppybin.buildRunConfig
// for parity between server-side probe and agent-side run. The DB URL is
// emitted as the literal sentinel "__DB_URL__"; a follow-up OneShot node
// (render-stroppy-config-url) sed-substitutes it with the runtime value
// pulled from the DagRun state-store under "db.url".
func renderStroppyConfig(db *catalogpb.Database, wl *catalogpb.Workload) ([]byte, error) {
	cfg := map[string]any{
		"version": "1",
	}
	scriptName := stroppyScriptName(wl.GetShape().GetScript())
	if scriptName != "" {
		cfg["script"] = scriptName
	}
	if sql := wl.GetShape().GetSql(); sql != "" {
		cfg["sql"] = sql
	}
	driverType := stroppyDriverType(db.GetKind())
	if driverType != "" {
		drv := map[string]any{
			"driver_type": driverType,
			"url":         "__DB_URL__",
		}
		if pool := wl.GetShape().GetLoad().GetVus(); pool > 0 {
			drv["pool"] = map[string]any{"max_conns": pool, "min_conns": pool}
		}
		cfg["drivers"] = map[string]any{"0": drv}
	}
	env := map[string]string{}
	if v := wl.GetShape().GetLoad().GetVus(); v > 0 {
		env["POOL_SIZE"] = fmt.Sprintf("%d", v)
	}
	if len(env) > 0 {
		cfg["env"] = env
	}
	return json.Marshal(cfg)
}

// stroppyScriptName maps Workload_Script enum to the canonical lowercase
// name the stroppy binary expects on its CLI.
func stroppyScriptName(s catalogpb.Workload_Script) string {
	switch s {
	case catalogpb.Workload_SCRIPT_TPCC_PROCS:
		return "tpcc/procs"
	case catalogpb.Workload_SCRIPT_TPCC_TX:
		return "tpcc/tx"
	case catalogpb.Workload_SCRIPT_TPCB_PROCS:
		return "tpcb/procs"
	case catalogpb.Workload_SCRIPT_TPCB_TX:
		return "tpcb/tx"
	case catalogpb.Workload_SCRIPT_TPCH_TX:
		return "tpch/tx"
	case catalogpb.Workload_SCRIPT_TPCC_TX_YDB_PGWIRE:
		return "tpcc/tx-ydb-pgwire"
	case catalogpb.Workload_SCRIPT_TPCB_TX_YDB_PGWIRE:
		return "tpcb/tx-ydb-pgwire"
	}
	return ""
}

// stroppyDriverType maps Database_Kind to the driver_type string the stroppy
// binary accepts in its config JSON.
func stroppyDriverType(kind catalogpb.Database_Kind) string {
	switch kind {
	case catalogpb.Database_DATABASE_KIND_POSTGRES, catalogpb.Database_DATABASE_KIND_COCKROACH:
		return "postgres"
	case catalogpb.Database_DATABASE_KIND_MYSQL:
		return "mysql"
	case catalogpb.Database_DATABASE_KIND_MARIADB:
		return "mariadb"
	case catalogpb.Database_DATABASE_KIND_PICODATA:
		return "picodata"
	case catalogpb.Database_DATABASE_KIND_YDB:
		return "ydb"
	case catalogpb.Database_DATABASE_KIND_YDB_MANAGED:
		return "ydb-managed"
	}
	return ""
}

// enginePlan captures the per-engine DAG decisions. New engines: extend
// planForEngine.
type enginePlan struct {
	module    taskspb.TerraformTask_Module
	installDB bool
	external  bool
	initCmd   string // empty = no init step
}

func planForEngine(kind catalogpb.Database_Kind) enginePlan {
	switch kind {
	case catalogpb.Database_DATABASE_KIND_POSTGRES:
		return enginePlan{module: taskspb.TerraformTask_MODULE_YANDEX, installDB: true}
	case catalogpb.Database_DATABASE_KIND_MYSQL,
		catalogpb.Database_DATABASE_KIND_MARIADB:
		return enginePlan{module: taskspb.TerraformTask_MODULE_YANDEX, installDB: true}
	case catalogpb.Database_DATABASE_KIND_PICODATA:
		return enginePlan{
			module:    taskspb.TerraformTask_MODULE_YANDEX,
			installDB: true,
			initCmd:   "picodata admin --instance-dir=/var/lib/picodata bootstrap",
		}
	case catalogpb.Database_DATABASE_KIND_COCKROACH:
		return enginePlan{
			module:    taskspb.TerraformTask_MODULE_YANDEX,
			installDB: true,
			initCmd:   "cockroach init --certs-dir=/var/lib/cockroach/certs",
		}
	case catalogpb.Database_DATABASE_KIND_YDB:
		return enginePlan{
			module:    taskspb.TerraformTask_MODULE_YANDEX,
			installDB: true,
			initCmd:   "ydbd admin blobstorage init --config /etc/ydbd/config.yaml",
		}
	case catalogpb.Database_DATABASE_KIND_YDB_MANAGED:
		return enginePlan{
			module:    taskspb.TerraformTask_MODULE_YANDEX_MANAGED_YDB,
			installDB: false,
		}
	}
	return enginePlan{}
}

// supported is true when the Database_Kind has an explicit branch in
// planForEngine above.
func supported(kind catalogpb.Database_Kind) bool {
	switch kind {
	case catalogpb.Database_DATABASE_KIND_POSTGRES,
		catalogpb.Database_DATABASE_KIND_MYSQL,
		catalogpb.Database_DATABASE_KIND_MARIADB,
		catalogpb.Database_DATABASE_KIND_PICODATA,
		catalogpb.Database_DATABASE_KIND_YDB,
		catalogpb.Database_DATABASE_KIND_YDB_MANAGED,
		catalogpb.Database_DATABASE_KIND_COCKROACH:
		return true
	}
	return false
}

func mkNode(id string, deps []string, spec proto.Message) (*systempb.Dag_Node, error) {
	a, err := anypb.New(spec)
	if err != nil {
		return nil, err
	}
	return &systempb.Dag_Node{
		Id:   id,
		Type: a.GetTypeUrl(),
		Deps: deps,
		Spec: a,
	}, nil
}

// resolveDatabase returns a concrete *catalogpb.Database from the DatabaseOrPreset oneof.
func (b *Builder) resolveDatabase(ctx context.Context, dop *catalogpb.DatabaseOrPreset) (*catalogpb.Database, error) {
	if dop == nil {
		return nil, fmt.Errorf("dagbuilder: TestRun.Database is nil")
	}
	if inline := dop.GetDatabase(); inline != nil {
		return inline, nil
	}
	if pid := dop.GetDatabasePresetId(); pid != nil {
		p, err := b.catalog.GetDatabasePreset(ctx, pid)
		if err != nil {
			return nil, err
		}
		return p.GetDatabase(), nil
	}
	return nil, fmt.Errorf("dagbuilder: DatabaseOrPreset oneof is empty")
}

// resolveWorkload returns a concrete *catalogpb.Workload from the WorkloadOrPreset oneof.
func (b *Builder) resolveWorkload(ctx context.Context, wop *catalogpb.WorkloadOrPreset) (*catalogpb.Workload, error) {
	if wop == nil {
		return nil, fmt.Errorf("dagbuilder: TestRun.Workload is nil")
	}
	if inline := wop.GetWorkload(); inline != nil {
		return inline, nil
	}
	if pid := wop.GetWorkloadPresetId(); pid != nil {
		p, err := b.catalog.GetWorkloadPreset(ctx, pid)
		if err != nil {
			return nil, err
		}
		return p.GetWorkload(), nil
	}
	return nil, fmt.Errorf("dagbuilder: WorkloadOrPreset oneof is empty")
}
