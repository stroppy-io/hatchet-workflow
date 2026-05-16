package dagbuilder_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// fakeCatalog satisfies testing.CatalogPort.
type fakeCatalog struct{}

func (fakeCatalog) GetDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId) (*catalogpb.DatabasePreset, error) {
	return &catalogpb.DatabasePreset{
		Database: &catalogpb.Database{},
	}, nil
}

func (fakeCatalog) GetWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId) (*catalogpb.WorkloadPreset, error) {
	return &catalogpb.WorkloadPreset{
		Workload: &catalogpb.Workload{},
	}, nil
}

func (fakeCatalog) GetPackage(ctx context.Context, id *catalogpb.PackageId) (*catalogpb.Package, error) {
	return &catalogpb.Package{}, nil
}

func TestBuilder_FromTestRun_InlineDatabaseWorkload(t *testing.T) {
	b := dagbuilder.New(fakeCatalog{})
	tr := &testingpb.TestRun{
		Database: &catalogpb.DatabaseOrPreset{
			DatabaseVariant: &catalogpb.DatabaseOrPreset_Database{
				Database: &catalogpb.Database{},
			},
		},
		Workload: &catalogpb.WorkloadOrPreset{
			WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{
				Workload: &catalogpb.Workload{},
			},
		},
	}

	dag, err := b.FromTestRun(context.Background(), tr)
	require.NoError(t, err)
	require.NotNil(t, dag.GetGraph())
	require.Len(t, dag.GetGraph().GetNodes(), 4)

	type nodeInfo struct {
		Deps []string
	}
	byID := map[string]*nodeInfo{}
	for _, n := range dag.GetGraph().GetNodes() {
		byID[n.GetId()] = &nodeInfo{Deps: n.GetDeps()}
	}

	require.Contains(t, byID, "provision")
	require.Empty(t, byID["provision"].Deps)
	require.Equal(t, []string{"provision"}, byID["install"].Deps)
	require.Equal(t, []string{"install"}, byID["workload"].Deps)
	require.Equal(t, []string{"workload"}, byID["teardown"].Deps)
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
	require.Len(t, dag.GetGraph().GetNodes(), 4)
}

func TestBuilder_FromTestRun_NilDatabase(t *testing.T) {
	b := dagbuilder.New(fakeCatalog{})
	tr := &testingpb.TestRun{
		Database: nil,
		Workload: &catalogpb.WorkloadOrPreset{
			WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{
				Workload: &catalogpb.Workload{},
			},
		},
	}

	_, err := b.FromTestRun(context.Background(), tr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Database is nil")
}
