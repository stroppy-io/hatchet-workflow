package testing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
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
