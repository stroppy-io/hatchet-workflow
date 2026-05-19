package dagbuilder_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// fakeCatalog satisfies dagbuilder.CatalogPort.
type fakeCatalog struct{}

func (fakeCatalog) GetDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId) (*catalogpb.DatabasePreset, error) {
	return &catalogpb.DatabasePreset{
		Database: &catalogpb.Database{
			Kind: catalogpb.Database_DATABASE_KIND_POSTGRES,
			Variant: &catalogpb.Database_Postgres_{Postgres: &catalogpb.Database_Postgres{
				Shape: &catalogpb.Database_Postgres_Shape{Replicas: 1},
			}},
		},
	}, nil
}

func (fakeCatalog) GetWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId) (*catalogpb.WorkloadPreset, error) {
	return &catalogpb.WorkloadPreset{Workload: &catalogpb.Workload{}}, nil
}

func (fakeCatalog) GetPackage(ctx context.Context, id *catalogpb.PackageId) (*catalogpb.Package, error) {
	return &catalogpb.Package{}, nil
}

func (fakeCatalog) ListPackages(_ context.Context, _ *iampb.TenantId, dbKind *catalogpb.Database_Kind) ([]*catalogpb.Package, error) {
	pgPkg := &catalogpb.Package{
		Identity: &commonpb.Identity{Name: "PostgreSQL 16"},
		DbKind:   catalogpb.Database_DATABASE_KIND_POSTGRES,
		Source: &catalogpb.Package_PackageSource{
			Source: &catalogpb.Package_PackageSource_Apt{
				Apt: &catalogpb.Package_AptSource{AptPackages: []string{"postgresql-16"}},
			},
		},
	}
	monPkg := &catalogpb.Package{
		Identity: &commonpb.Identity{Name: "Monitor Agents"},
		DbKind:   catalogpb.Database_DATABASE_KIND_UNSPECIFIED,
		Source: &catalogpb.Package_PackageSource{
			Source: &catalogpb.Package_PackageSource_Apt{
				Apt: &catalogpb.Package_AptSource{AptPackages: []string{"prometheus-node-exporter"}},
			},
		},
	}
	if dbKind != nil {
		switch *dbKind {
		case catalogpb.Database_DATABASE_KIND_POSTGRES:
			return []*catalogpb.Package{pgPkg}, nil
		}
		return nil, nil
	}
	return []*catalogpb.Package{pgPkg, monPkg}, nil
}

func postgresInlineRun() *testingpb.TestRun {
	return &testingpb.TestRun{
		Database: &catalogpb.DatabaseOrPreset{
			DatabaseVariant: &catalogpb.DatabaseOrPreset_Database{
				Database: &catalogpb.Database{
					Kind: catalogpb.Database_DATABASE_KIND_POSTGRES,
					Variant: &catalogpb.Database_Postgres_{Postgres: &catalogpb.Database_Postgres{
						Shape: &catalogpb.Database_Postgres_Shape{Replicas: 1},
					}},
				},
			},
		},
		Workload: &catalogpb.WorkloadOrPreset{
			WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{Workload: &catalogpb.Workload{}},
		},
	}
}

func TestBuilder_FromTestRun_InlineDatabaseWorkload(t *testing.T) {
	b := dagbuilder.New(fakeCatalog{})
	dag, err := b.FromTestRun(context.Background(), postgresInlineRun())
	require.NoError(t, err)
	require.NotNil(t, dag.GetGraph())

	byID := map[string]*dagNode{}
	for _, n := range dag.GetGraph().GetNodes() {
		byID[n.GetId()] = &dagNode{Deps: n.GetDeps()}
	}

	// Per-machine targeting DAG: provision, wait-agents, install-db,
	// install-monitor, config-db, render-stroppy-config, stroppy-run, teardown.
	require.Contains(t, byID, "provision")
	require.Empty(t, byID["provision"].Deps)
	require.Equal(t, []string{"provision"}, byID["wait-agents"].Deps)
	require.Equal(t, []string{"wait-agents"}, byID["install-db"].Deps)
	require.Equal(t, []string{"wait-agents"}, byID["install-monitor"].Deps)
	require.Equal(t, []string{"install-db"}, byID["config-db"].Deps)
	require.Equal(t, []string{"config-db"}, byID["render-stroppy-config"].Deps)
	require.Equal(t, []string{"render-stroppy-config"}, byID["render-stroppy-config-url"].Deps)
	require.Equal(t, []string{"render-stroppy-config-url"}, byID["stroppy-run"].Deps)
	require.Equal(t, []string{"stroppy-run"}, byID["teardown"].Deps)
}

func TestBuilder_FromTestRun_PresetDatabase(t *testing.T) {
	b := dagbuilder.New(fakeCatalog{})
	tr := &testingpb.TestRun{
		Database: &catalogpb.DatabaseOrPreset{
			DatabaseVariant: &catalogpb.DatabaseOrPreset_DatabasePresetId{
				DatabasePresetId: &catalogpb.DatabasePresetId{Value: "preset-1"},
			},
		},
		Workload: &catalogpb.WorkloadOrPreset{
			WorkloadVariant: &catalogpb.WorkloadOrPreset_WorkloadPresetId{
				WorkloadPresetId: &catalogpb.WorkloadPresetId{Value: "preset-w1"},
			},
		},
	}
	dag, err := b.FromTestRun(context.Background(), tr)
	require.NoError(t, err)
	require.NotNil(t, dag.GetGraph())
	require.NotEmpty(t, dag.GetGraph().GetNodes())
}

func TestBuilder_FromTestRun_NilDatabase(t *testing.T) {
	b := dagbuilder.New(fakeCatalog{})
	tr := &testingpb.TestRun{
		Database: nil,
		Workload: &catalogpb.WorkloadOrPreset{
			WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{Workload: &catalogpb.Workload{}},
		},
	}
	_, err := b.FromTestRun(context.Background(), tr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Database is nil")
}

type dagNode struct{ Deps []string }
