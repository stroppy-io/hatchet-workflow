package dagbuilder

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
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
}

// Builder converts a TestRun into a system.Dag template.
type Builder struct{ catalog CatalogPort }

// New wires the builder against a CatalogPort.
func New(catalog CatalogPort) *Builder { return &Builder{catalog: catalog} }

// FromTestRun expands a TestRun into a full per-engine DAG. The shape is:
//
//	provision         (Terraform OP_APPLY, module selected by engine)
//	install-db        (PackageInstall, role=DATABASE)
//	install-monitor   (PackageInstall, role=MONITOR — vmagent + node_exporter)
//	config-db         (ConfigApply, engine-specific templates)
//	init-db           (OneShot, engine bootstrap: cockroach init, ydbd init, etc.)
//	config-monitor    (ConfigApply, vmagent scrape config)
//	workload          (StroppyRun)
//	teardown          (Terraform OP_DESTROY)
//
// External-DB short-circuits to just config-db + workload (no infra).
// Managed-YDB uses MODULE_YANDEX_MANAGED_YDB and skips install-db / init-db
// (cloud manages the database).
func (b *Builder) FromTestRun(ctx context.Context, tr *testingpb.TestRun) (*systempb.Dag, error) {
	db, err := b.resolveDatabase(ctx, tr.GetDatabase())
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: resolve database: %w", err)
	}
	wl, err := b.resolveWorkload(ctx, tr.GetWorkload())
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: resolve workload: %w", err)
	}

	plan := planForEngine(db.GetKind())
	// External DB variant is a per-row attribute orthogonal to kind: the
	// Database message carries an oneof variant; when External is set we
	// skip all infra and just run the workload.
	if db.GetExternal() != nil {
		plan.external = true
	}
	module := plan.module

	var nodes []*systempb.Dag_Node

	if plan.external {
		// External DB: only a workload run, no infra.
		workload, err := mkNode("workload", nil, &taskspb.StroppyRunTask{
			Workload:      wl,
			DbUrlStateKey: "db.url",
		})
		if err != nil {
			return nil, fmt.Errorf("dagbuilder: workload node: %w", err)
		}
		return &systempb.Dag{Graph: &systempb.Dag_Graph{Nodes: []*systempb.Dag_Node{workload}}}, nil
	}

	provision, err := mkNode("provision", nil, &taskspb.TerraformTask{
		Op:           taskspb.TerraformTask_OP_APPLY,
		Module:       module,
		WdIdStateKey: "yc.wd",
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: provision node: %w", err)
	}
	nodes = append(nodes, provision)

	prev := []string{"provision"}

	if plan.installDB {
		installDB, err := mkNode("install-db", prev, &taskspb.PackageInstallTask{
			Role: catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
		})
		if err != nil {
			return nil, fmt.Errorf("dagbuilder: install-db node: %w", err)
		}
		nodes = append(nodes, installDB)

		installMon, err := mkNode("install-monitor", prev, &taskspb.PackageInstallTask{
			Role: catalogpb.MachineRole_MACHINE_ROLE_MONITOR,
		})
		if err != nil {
			return nil, fmt.Errorf("dagbuilder: install-monitor node: %w", err)
		}
		nodes = append(nodes, installMon)

		configDB, err := mkNode("config-db", []string{"install-db"}, &taskspb.ConfigApplyTask{
			EngineKind: db.GetKind(),
			Role:       catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
		})
		if err != nil {
			return nil, fmt.Errorf("dagbuilder: config-db node: %w", err)
		}
		nodes = append(nodes, configDB)

		if plan.initCmd != "" {
			initDB, err := mkNode("init-db", []string{"config-db"}, &taskspb.OneShotTask{
				EngineKind:      db.GetKind(),
				Role:            catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
				Target:          taskspb.OneShotTask_TARGET_FIRST,
				CommandTemplate: plan.initCmd,
			})
			if err != nil {
				return nil, fmt.Errorf("dagbuilder: init-db node: %w", err)
			}
			nodes = append(nodes, initDB)
			prev = []string{"init-db"}
		} else {
			prev = []string{"config-db"}
		}
	}

	workload, err := mkNode("workload", prev, &taskspb.StroppyRunTask{
		Workload:      wl,
		DbUrlStateKey: "db.url",
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: workload node: %w", err)
	}
	nodes = append(nodes, workload)

	teardown, err := mkNode("teardown", []string{"workload"}, &taskspb.TerraformTask{
		Op:           taskspb.TerraformTask_OP_DESTROY,
		Module:       module,
		WdIdStateKey: "yc.wd",
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: teardown node: %w", err)
	}
	nodes = append(nodes, teardown)

	return &systempb.Dag{Graph: &systempb.Dag_Graph{Nodes: nodes}}, nil
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
		// Managed YDB: separate TF module, cloud manages the database, so
		// we skip install + init. Only provision (client VM + managed db)
		// and workload run.
		return enginePlan{
			module:    taskspb.TerraformTask_MODULE_YANDEX_MANAGED_YDB,
			installDB: false,
		}
	}
	// Unknown engine: skeleton 4-node DAG.
	return enginePlan{module: taskspb.TerraformTask_MODULE_UNSPECIFIED, installDB: true}
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
