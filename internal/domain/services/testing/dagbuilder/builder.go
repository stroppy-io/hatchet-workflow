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

type Builder struct {
	catalog CatalogPort
}

func New(catalog CatalogPort) *Builder { return &Builder{catalog: catalog} }

// FromTestRun produces a 4-node DAG: provision → install → workload → teardown.
func (b *Builder) FromTestRun(ctx context.Context, tr *testingpb.TestRun) (*systempb.Dag, error) {
	db, err := b.resolveDatabase(ctx, tr.GetDatabase())
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: resolve database: %w", err)
	}
	wl, err := b.resolveWorkload(ctx, tr.GetWorkload())
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: resolve workload: %w", err)
	}

	// Pick terraform module from db.Kind; default to MODULE_UNSPECIFIED until full mapping is added.
	module := taskspb.TerraformTask_MODULE_UNSPECIFIED

	provision, err := mkNode("provision", nil, &taskspb.TerraformTask{
		Op:           taskspb.TerraformTask_OP_APPLY,
		Module:       module,
		WdIdStateKey: "yc.wd",
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: provision node: %w", err)
	}

	install, err := mkNode("install", []string{"provision"}, &taskspb.PackageInstallTask{
		// PackageId left nil; wire when TestRun gains an explicit Package reference.
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: install node: %w", err)
	}

	workload, err := mkNode("workload", []string{"install"}, &taskspb.StroppyRunTask{
		Workload:      wl,
		DbUrlStateKey: "db.url",
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: workload node: %w", err)
	}

	teardown, err := mkNode("teardown", []string{"workload"}, &taskspb.TerraformTask{
		Op:           taskspb.TerraformTask_OP_DESTROY,
		Module:       module,
		WdIdStateKey: "yc.wd",
	})
	if err != nil {
		return nil, fmt.Errorf("dagbuilder: teardown node: %w", err)
	}

	_ = db // consumed by TerraformTask.Vars in a later cut

	return &systempb.Dag{
		Graph: &systempb.Dag_Graph{
			Nodes: []*systempb.Dag_Node{provision, install, workload, teardown},
		},
	}, nil
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
	// Inline variant: DatabaseOrPreset_Database
	if inline := dop.GetDatabase(); inline != nil {
		return inline, nil
	}
	// Preset reference variant: DatabaseOrPreset_DatabasePresetId
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
	// Inline variant: WorkloadOrPreset_Workload
	if inline := wop.GetWorkload(); inline != nil {
		return inline, nil
	}
	// Preset reference variant: WorkloadOrPreset_WorkloadPresetId
	if pid := wop.GetWorkloadPresetId(); pid != nil {
		p, err := b.catalog.GetWorkloadPreset(ctx, pid)
		if err != nil {
			return nil, err
		}
		return p.GetWorkload(), nil
	}
	return nil, fmt.Errorf("dagbuilder: WorkloadOrPreset oneof is empty")
}
