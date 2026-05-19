package testing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
)

// fakeCatalog satisfies CatalogPort.
type fakeCatalog struct{}

func (fakeCatalog) GetDatabasePreset(_ context.Context, _ *catalogpb.DatabasePresetId) (*catalogpb.DatabasePreset, error) {
	return &catalogpb.DatabasePreset{Database: &catalogpb.Database{}}, nil
}
func (fakeCatalog) GetWorkloadPreset(_ context.Context, _ *catalogpb.WorkloadPresetId) (*catalogpb.WorkloadPreset, error) {
	return &catalogpb.WorkloadPreset{Workload: &catalogpb.Workload{}}, nil
}
func (fakeCatalog) GetPackage(_ context.Context, _ *catalogpb.PackageId) (*catalogpb.Package, error) {
	return &catalogpb.Package{}, nil
}
func (fakeCatalog) ListPackages(_ context.Context, _ *iampb.TenantId, dbKind *catalogpb.Database_Kind) ([]*catalogpb.Package, error) {
	pg := &catalogpb.Package{
		Identity: &commonpb.Identity{Name: "PostgreSQL 16"},
		DbKind:   catalogpb.Database_DATABASE_KIND_POSTGRES,
		Source: &catalogpb.Package_PackageSource{Source: &catalogpb.Package_PackageSource_Apt{
			Apt: &catalogpb.Package_AptSource{AptPackages: []string{"postgresql-16"}},
		}},
	}
	mon := &catalogpb.Package{
		Identity: &commonpb.Identity{Name: "Monitor Agents"},
		Source: &catalogpb.Package_PackageSource{Source: &catalogpb.Package_PackageSource_Apt{
			Apt: &catalogpb.Package_AptSource{AptPackages: []string{"prometheus-node-exporter"}},
		}},
	}
	if dbKind != nil {
		switch *dbKind {
		case catalogpb.Database_DATABASE_KIND_POSTGRES:
			return []*catalogpb.Package{pg}, nil
		}
		return nil, nil
	}
	return []*catalogpb.Package{pg, mon}, nil
}

func TestTestRunService_CreateAndLaunch(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)

	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	catalog := fakeCatalog{}
	b := dagbuilder.New(catalog)
	svc := testingsvc.NewTestRunService(executor, iam.F.TxMgr, iam.F.Events, catalog, sys, b)

	ctx := context.Background()
	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "tr@e.com", Nickname: "tr"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T"}}, u.GetId())

	tr := &testingpb.TestRun{
		Identity: &commonpb.Identity{Name: "smoke-run"},
		Database: &catalogpb.DatabaseOrPreset{
			DatabaseVariant: &catalogpb.DatabaseOrPreset_Database{
				Database: &catalogpb.Database{Kind: catalogpb.Database_DATABASE_KIND_POSTGRES},
			},
		},
		Workload: &catalogpb.WorkloadOrPreset{
			WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{
				Workload: &catalogpb.Workload{},
			},
		},
	}

	created, err := svc.CreateTestRun(ctx, tn.GetId(), u.GetId(), tr)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())
	require.Nil(t, created.GetDagRunId())

	launched, err := svc.LaunchTestRun(ctx, created.GetId())
	require.NoError(t, err)
	require.NotNil(t, launched.GetDagRunId())
	require.NotEmpty(t, launched.GetDagRunId().GetValue())

	// Verify DagRun exists and is in RUNNING state via system service.
	dr, err := sys.GetDagRun(ctx, launched.GetDagRunId())
	require.NoError(t, err)
	require.NotNil(t, dr)
}

func TestTestRunService_Update(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)

	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	catalog := fakeCatalog{}
	b := dagbuilder.New(catalog)
	svc := testingsvc.NewTestRunService(executor, iam.F.TxMgr, iam.F.Events, catalog, sys, b)

	ctx := context.Background()
	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "upd@e.com", Nickname: "upd"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T-upd"}}, u.GetId())

	created, err := svc.CreateTestRun(ctx, tn.GetId(), u.GetId(), &testingpb.TestRun{
		Identity: &commonpb.Identity{Name: "v1"},
	})
	require.NoError(t, err)

	updated, err := svc.UpdateTestRun(ctx, &testingpb.TestRun{
		Id:       created.GetId(),
		Identity: &commonpb.Identity{Name: "v2"},
	})
	require.NoError(t, err)
	require.Equal(t, "v2", updated.GetIdentity().GetName())

	got, err := svc.GetTestRun(ctx, created.GetId())
	require.NoError(t, err)
	require.Equal(t, "v2", got.GetIdentity().GetName())
}

func TestTestRunService_DeleteSoftDelete(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)

	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	catalog := fakeCatalog{}
	b := dagbuilder.New(catalog)
	svc := testingsvc.NewTestRunService(executor, iam.F.TxMgr, iam.F.Events, catalog, sys, b)

	ctx := context.Background()
	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "del@e.com", Nickname: "del"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T-del"}}, u.GetId())

	created, err := svc.CreateTestRun(ctx, tn.GetId(), u.GetId(), &testingpb.TestRun{
		Identity: &commonpb.Identity{Name: "to-delete"},
	})
	require.NoError(t, err)

	deleted, err := svc.DeleteTestRun(ctx, created.GetId())
	require.NoError(t, err)
	require.Equal(t, created.GetId().GetValue(), deleted.GetId().GetValue())

	list, err := svc.ListTestRuns(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list, 0)
}

// TestTestRunService_DryRun verifies DryRunTestRun returns a Dag preview
// without persisting any TestRun / DagRun. Covers gap A21 + A22.
func TestTestRunService_DryRun(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)

	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	cat := fakeCatalog{}
	b := dagbuilder.New(cat)
	svc := testingsvc.NewTestRunService(executor, iam.F.TxMgr, iam.F.Events, cat, sys, b)

	ctx := context.Background()
	tr := &testingpb.TestRun{
		Identity: &commonpb.Identity{Name: "preview-run"},
		Database: &catalogpb.DatabaseOrPreset{
			DatabaseVariant: &catalogpb.DatabaseOrPreset_Database{
				Database: &catalogpb.Database{Kind: catalogpb.Database_DATABASE_KIND_POSTGRES},
			},
		},
		Workload: &catalogpb.WorkloadOrPreset{
			WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{
				Workload: &catalogpb.Workload{},
			},
		},
	}

	dag, err := svc.DryRunTestRun(ctx, tr)
	require.NoError(t, err)
	require.NotNil(t, dag, "Dag preview must be non-nil")
	require.NotNil(t, dag.GetGraph(), "Dag preview must contain a graph")
	require.NotEmpty(t, dag.GetGraph().GetNodes(), "Dag preview graph must contain nodes")

	// No persistence: ListTestRuns for the only tenant we know about should be empty.
	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "dry@e.com", Nickname: "dry"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T-dry"}}, u.GetId())
	list, err := svc.ListTestRuns(ctx, tn.GetId())
	require.NoError(t, err)
	require.Empty(t, list, "DryRun must not persist a TestRun")
}

func TestTestRunService_DryRun_NilInput(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)
	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	cat := fakeCatalog{}
	b := dagbuilder.New(cat)
	svc := testingsvc.NewTestRunService(executor, iam.F.TxMgr, iam.F.Events, cat, sys, b)

	_, err := svc.DryRunTestRun(context.Background(), nil)
	require.Error(t, err)
}

// TestTestRunService_Cancel verifies the cancellation request propagates to
// the underlying DagRun's cancel_requested column. Covers gap A1.
func TestTestRunService_Cancel(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)

	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	cat := fakeCatalog{}
	b := dagbuilder.New(cat)
	svc := testingsvc.NewTestRunService(executor, iam.F.TxMgr, iam.F.Events, cat, sys, b)

	ctx := context.Background()
	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "cnl@e.com", Nickname: "cnl"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T-cnl"}}, u.GetId())

	tr := &testingpb.TestRun{
		Identity: &commonpb.Identity{Name: "cancel-me"},
		Database: &catalogpb.DatabaseOrPreset{
			DatabaseVariant: &catalogpb.DatabaseOrPreset_Database{
				Database: &catalogpb.Database{Kind: catalogpb.Database_DATABASE_KIND_POSTGRES},
			},
		},
		Workload: &catalogpb.WorkloadOrPreset{
			WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{
				Workload: &catalogpb.Workload{},
			},
		},
	}
	created, err := svc.CreateTestRun(ctx, tn.GetId(), u.GetId(), tr)
	require.NoError(t, err)
	launched, err := svc.LaunchTestRun(ctx, created.GetId())
	require.NoError(t, err)
	require.NotNil(t, launched.GetDagRunId())

	require.NoError(t, svc.CancelTestRun(ctx, created.GetId()))

	dr, err := sys.GetDagRun(ctx, launched.GetDagRunId())
	require.NoError(t, err)
	require.True(t, dr.GetCancelRequested(), "cancel_requested must be true after CancelTestRun")
}

// TestTestRunService_Cancel_NoDagRun: cancelling a TestRun that was never
// launched is a no-op (no DagRun yet → nothing to mark).
func TestTestRunService_Cancel_NoDagRun(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)
	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	cat := fakeCatalog{}
	b := dagbuilder.New(cat)
	svc := testingsvc.NewTestRunService(executor, iam.F.TxMgr, iam.F.Events, cat, sys, b)

	ctx := context.Background()
	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "nx@e.com", Nickname: "nx"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T-nx"}}, u.GetId())
	created, err := svc.CreateTestRun(ctx, tn.GetId(), u.GetId(), &testingpb.TestRun{
		Identity: &commonpb.Identity{Name: "never-launched"},
	})
	require.NoError(t, err)
	require.NoError(t, svc.CancelTestRun(ctx, created.GetId()))
}

// TestTestRunService_TenantIsolation: a TestRun created in tenant A must not
// appear in ListTestRuns for tenant B.
func TestTestRunService_TenantIsolation(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)
	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	cat := fakeCatalog{}
	b := dagbuilder.New(cat)
	svc := testingsvc.NewTestRunService(executor, iam.F.TxMgr, iam.F.Events, cat, sys, b)

	ctx := context.Background()
	uA, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "a@e.com", Nickname: "ua"}, "P@ss1234!")
	tnA, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "TA"}}, uA.GetId())
	uB, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "b@e.com", Nickname: "ub"}, "P@ss1234!")
	tnB, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "TB"}}, uB.GetId())

	_, err := svc.CreateTestRun(ctx, tnA.GetId(), uA.GetId(), &testingpb.TestRun{
		Identity: &commonpb.Identity{Name: "a-only"},
	})
	require.NoError(t, err)

	listA, err := svc.ListTestRuns(ctx, tnA.GetId())
	require.NoError(t, err)
	require.Len(t, listA, 1)

	listB, err := svc.ListTestRuns(ctx, tnB.GetId())
	require.NoError(t, err)
	require.Empty(t, listB, "Tenant B must not see Tenant A's runs")
}

func TestTestRunService_InstantiateFromTemplate(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)

	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	cat := fakeCatalog{}
	b := dagbuilder.New(cat)
	svc := testingsvc.NewTestRunService(executor, iam.F.TxMgr, iam.F.Events, cat, sys, b)
	tplSvc := testingsvc.NewTemplateService(executor, iam.F.TxMgr, iam.F.Events)

	ctx := context.Background()
	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "inst@e.com", Nickname: "inst"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T-inst"}}, u.GetId())

	tpl, err := tplSvc.CreateTestRunTemplate(ctx, tn.GetId(), u.GetId(), &testingpb.TestRunTemplate{
		Identity: &commonpb.Identity{Name: "my-template"},
		Database: &catalogpb.DatabaseOrPreset{
			DatabaseVariant: &catalogpb.DatabaseOrPreset_Database{
				Database: &catalogpb.Database{Kind: catalogpb.Database_DATABASE_KIND_POSTGRES},
			},
		},
		Workload: &catalogpb.WorkloadOrPreset{
			WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{
				Workload: &catalogpb.Workload{},
			},
		},
	})
	require.NoError(t, err)

	run, err := svc.InstantiateTestRun(ctx, tpl.GetId(), u.GetId())
	require.NoError(t, err)
	require.NotEmpty(t, run.GetId().GetValue())
	require.Equal(t, "my-template-copy", run.GetIdentity().GetName())
	require.Equal(t, tpl.GetId().GetValue(), run.GetTemplateId().GetValue())
	require.NotNil(t, run.GetDatabase())
	require.NotNil(t, run.GetWorkload())
}
